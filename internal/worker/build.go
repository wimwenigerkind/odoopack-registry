package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/auth"
	"github.com/wimwenigerkind/odoopack-registry/internal/git"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
	"github.com/wimwenigerkind/odoopack-registry/internal/storage"
)

type SyncJob struct {
	AddonID       uuid.UUID
	Name          string
	GitURL        string
	DefaultBranch string
	Subpath       string
	Trigger       string
	IntegrationID *uuid.UUID
}

type Queue struct {
	jobs            chan SyncJob
	versionRepo     *repository.AddonVersionRepository
	integrationRepo *repository.IntegrationRepository
	authRegistry    *auth.Registry
	storage         storage.Storage
	wg              sync.WaitGroup
}

func NewQueue(versionRepo *repository.AddonVersionRepository, integrationRepo *repository.IntegrationRepository, authRegistry *auth.Registry, store storage.Storage, bufferSize int) *Queue {
	return &Queue{
		jobs:            make(chan SyncJob, bufferSize),
		versionRepo:     versionRepo,
		integrationRepo: integrationRepo,
		authRegistry:    authRegistry,
		storage:         store,
	}
}

func (q *Queue) Start(ctx context.Context, workers int) {
	for i := range workers {
		q.wg.Add(1)
		go q.run(ctx, i)
	}
}

func (q *Queue) Enqueue(job SyncJob) {
	q.jobs <- job
}

func (q *Queue) Stop() {
	close(q.jobs)
	q.wg.Wait()
}

func (q *Queue) run(ctx context.Context, id int) {
	defer q.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-q.jobs:
			if !ok {
				return
			}
			q.process(ctx, id, job)
		}
	}
}

type desiredVersion struct {
	Version string
	Ref     git.Ref
}

func (q *Queue) process(ctx context.Context, workerID int, job SyncJob) {
	log := slog.With("worker", workerID, "addon", job.Name)
	log.Info("sync", "git_url", job.GitURL, "trigger", job.Trigger)

	cloneURL, err := q.resolveCloneURL(ctx, job)
	if err != nil {
		log.Error("resolve clone url", "err", err)
		return
	}

	refs, err := git.LsRemote(cloneURL)
	if err != nil {
		log.Error("ls-remote", "err", err)
		return
	}

	desired := planVersions(refs)
	if len(desired) == 0 {
		log.Info("no buildable refs")
		return
	}

	for _, d := range desired {
		if err := q.buildVersion(ctx, job, cloneURL, d); err != nil {
			log.Error("build", "version", d.Version, "err", err)
		}
	}
}

func planVersions(refs []git.Ref) []desiredVersion {
	var out []desiredVersion
	for _, r := range refs {
		switch r.Type {
		case git.RefTag:
			v, ok := normalizeVersion(r.Name)
			if !ok {
				slog.Warn("skipping non-version tag", "tag", r.Name)
				continue
			}
			out = append(out, desiredVersion{Version: v, Ref: r})
		case git.RefBranch:
			out = append(out, desiredVersion{Version: "dev-" + r.Name, Ref: r})
		}
	}
	return out
}

func (q *Queue) buildVersion(ctx context.Context, job SyncJob, cloneURL string, d desiredVersion) error {
	refType := models.RefTypeTag
	if d.Ref.Type == git.RefBranch {
		refType = models.RefTypeBranch
	}

	existing, err := q.versionRepo.Get(job.AddonID, d.Version)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return fmt.Errorf("lookup existing: %w", err)
	}
	if existing != nil {
		if refType == models.RefTypeTag && existing.Status == models.StatusReady && existing.RefValue != d.Ref.SHA {
			slog.Warn("tag moved after publish, keeping immutable version",
				"addon", job.Name, "version", d.Version,
				"published_sha", existing.RefValue, "new_sha", d.Ref.SHA)
			return nil
		}
		if existing.Status == models.StatusReady && existing.RefValue == d.Ref.SHA {
			if rc, err := q.storage.Get(ctx, existing.StorageKey); err == nil {
				rc.Close()
				return nil
			}
			slog.Warn("artifact missing, rebuilding", "addon", job.Name, "version", d.Version, "storage_key", existing.StorageKey)
		}
	}

	av := &models.AddonVersion{
		AddonID:  job.AddonID,
		Version:  d.Version,
		RefType:  refType,
		RefValue: d.Ref.SHA,
		Status:   models.StatusBuilding,
	}
	if err := q.versionRepo.Upsert(av); err != nil {
		return fmt.Errorf("mark building: %w", err)
	}

	rootDir := moduleDir(job.Name, job.Subpath)
	archive, err := git.CloneAndZip(cloneURL, d.Ref.Name, rootDir, job.Subpath)
	if err != nil {
		_ = q.versionRepo.UpdateStatus(av.ID, models.StatusFailed, auth.RedactURLCredentials(err.Error()))
		return err
	}
	defer os.Remove(archive.ZipPath)

	f, err := os.Open(archive.ZipPath)
	if err != nil {
		_ = q.versionRepo.UpdateStatus(av.ID, models.StatusFailed, auth.RedactURLCredentials(err.Error()))
		return err
	}
	defer f.Close()

	keyName := strings.ReplaceAll(d.Version+"-"+d.Ref.SHA, "/", "-")
	key := fmt.Sprintf("packages/%s/%s.zip", job.AddonID, keyName)
	if err := q.storage.Put(ctx, key, f, storage.PutOptions{ContentType: "application/zip"}); err != nil {
		_ = q.versionRepo.UpdateStatus(av.ID, models.StatusFailed, auth.RedactURLCredentials(err.Error()))
		return err
	}

	if err := q.versionRepo.SetReady(av.ID, key, "sha256:"+archive.ContentHash, archive.SizeBytes); err != nil {
		return fmt.Errorf("mark ready: %w", err)
	}
	slog.Info("built", "addon", job.Name, "version", d.Version, "ref", d.Ref.SHA, "bytes", archive.SizeBytes)
	return nil
}

var versionRe = regexp.MustCompile(`^[0-9]+(\.[0-9]+)*([-+][0-9A-Za-z.-]+)?$`)

func normalizeVersion(tag string) (string, bool) {
	v := strings.TrimSpace(tag)
	if v != "" && (v[0] == 'v' || v[0] == 'V') {
		v = v[1:]
	}
	if v == "" || !versionRe.MatchString(v) {
		return "", false
	}
	return v, true
}

func moduleDir(name, subpath string) string {
	if cleaned := strings.Trim(subpath, "/"); cleaned != "" {
		return path.Base(cleaned)
	}
	if i := strings.LastIndex(name, "/"); i >= 0 {
		return name[i+1:]
	}
	return name
}
