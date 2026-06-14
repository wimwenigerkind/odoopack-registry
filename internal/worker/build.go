package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
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
	for i := 0; i < workers; i++ {
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

	desired := planVersions(refs, job.DefaultBranch)
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

func planVersions(refs []git.Ref, defaultBranch string) []desiredVersion {
	var out []desiredVersion
	for _, r := range refs {
		if r.Type == git.RefTag {
			out = append(out, desiredVersion{Version: r.Name, Ref: r})
		}
		if r.Type == git.RefBranch && r.Name == defaultBranch {
			out = append(out, desiredVersion{Version: "latest", Ref: r})
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
	if existing != nil && existing.Status == models.StatusReady && existing.RefValue == d.Ref.SHA {
		rc, err := q.storage.Get(ctx, existing.StorageKey)
		if err == nil {
			rc.Close()
			return nil
		}
		slog.Warn("storage object missing, rebuilding", "addon", job.Name, "version", d.Version, "storage_key", existing.StorageKey)
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

	rootDir := sanitize(job.Name) + "-" + sanitize(d.Version)
	archive, err := git.CloneAndZip(cloneURL, d.Ref.Name, rootDir)
	if err != nil {
		_ = q.versionRepo.UpdateStatus(av.ID, models.StatusFailed, err.Error())
		return err
	}
	defer os.Remove(archive.ZipPath)

	f, err := os.Open(archive.ZipPath)
	if err != nil {
		_ = q.versionRepo.UpdateStatus(av.ID, models.StatusFailed, err.Error())
		return err
	}
	defer f.Close()

	key := fmt.Sprintf("packages/%s/%s.zip", job.AddonID, sanitize(d.Version))
	if err := q.storage.Put(ctx, key, f, storage.PutOptions{ContentType: "application/zip"}); err != nil {
		_ = q.versionRepo.UpdateStatus(av.ID, models.StatusFailed, err.Error())
		return err
	}

	if err := q.versionRepo.SetReady(av.ID, key, "sha256:"+archive.ContentHash, archive.SizeBytes); err != nil {
		return fmt.Errorf("mark ready: %w", err)
	}
	slog.Info("built", "addon", job.Name, "version", d.Version, "bytes", archive.SizeBytes)
	return nil
}

func sanitize(s string) string {
	return strings.ReplaceAll(s, "/", "_")
}
