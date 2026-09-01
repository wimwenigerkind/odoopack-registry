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
	"time"

	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/auth"
	"github.com/wimwenigerkind/odoopack-registry/internal/git"
	"github.com/wimwenigerkind/odoopack-registry/internal/markdown"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
	"github.com/wimwenigerkind/odoopack-registry/internal/storage"
	"gorm.io/gorm"
)

type Consumer struct {
	db              *gorm.DB
	versionRepo     *repository.AddonVersionRepository
	integrationRepo *repository.IntegrationRepository
	authRegistry    *auth.Registry
	storage         storage.Storage
	id              string
	pollInterval    time.Duration
	wg              sync.WaitGroup
}

func NewConsumer(db *gorm.DB, versionRepo *repository.AddonVersionRepository, integrationRepo *repository.IntegrationRepository, authRegistry *auth.Registry, store storage.Storage, pollInterval time.Duration) *Consumer {
	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}
	return &Consumer{
		db:              db,
		versionRepo:     versionRepo,
		integrationRepo: integrationRepo,
		authRegistry:    authRegistry,
		storage:         store,
		id:              "worker-" + uuid.NewString(),
		pollInterval:    pollInterval,
	}
}

func (c *Consumer) Run(ctx context.Context, workers int) {
	for i := range workers {
		c.wg.Add(1)
		go c.loop(ctx, i)
	}
}

func (c *Consumer) Wait() {
	c.wg.Wait()
}

func (c *Consumer) loop(ctx context.Context, id int) {
	defer c.wg.Done()
	log := slog.With("worker", id)
	t := time.NewTicker(c.pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			for {
				job, ok, err := c.claim(ctx)
				if err != nil {
					if ctx.Err() == nil {
						log.Error("claim job", "err", err)
					}
					break
				}
				if !ok {
					break
				}
				c.handle(ctx, log, job)
			}
		}
	}
}

func (c *Consumer) claim(ctx context.Context) (models.SyncJob, bool, error) {
	var job models.SyncJob
	err := c.db.WithContext(ctx).Raw(`
UPDATE sync_jobs SET status = ?, locked_at = now(), locked_by = ?, updated_at = now()
WHERE id = (
	SELECT id FROM sync_jobs
	WHERE status = ? AND run_after <= now()
	ORDER BY created_at
	FOR UPDATE SKIP LOCKED
	LIMIT 1
)
RETURNING *`, models.SyncJobRunning, c.id, models.SyncJobPending).Scan(&job).Error
	if err != nil {
		return models.SyncJob{}, false, err
	}
	if job.ID == uuid.Nil {
		return models.SyncJob{}, false, nil
	}
	return job, true, nil
}

func (c *Consumer) handle(ctx context.Context, log *slog.Logger, job models.SyncJob) {
	log = log.With("addon", job.Name, "job", job.ID, "trigger", job.Trigger)
	log.Info("sync start", "git_url", job.GitURL)

	if err := c.process(ctx, log, job); err != nil {
		attempts := job.Attempts + 1
		updates := map[string]any{
			"attempts":   attempts,
			"last_error": auth.RedactURLCredentials(err.Error()),
			"locked_at":  nil,
			"locked_by":  "",
			"updated_at": time.Now(),
		}
		if attempts >= job.MaxAttempts {
			updates["status"] = models.SyncJobFailed
			log.Error("sync failed permanently", "attempts", attempts, "err", err)
		} else {
			updates["status"] = models.SyncJobPending
			updates["run_after"] = time.Now().Add(time.Duration(attempts) * 30 * time.Second)
			log.Warn("sync failed, will retry", "attempts", attempts, "err", err)
		}
		c.db.Model(&models.SyncJob{}).Where("id = ?", job.ID).Updates(updates)
		return
	}

	c.db.Where("id = ?", job.ID).Delete(&models.SyncJob{})
	log.Info("sync done")
}

func (c *Consumer) process(ctx context.Context, log *slog.Logger, job models.SyncJob) error {
	cloneURL, err := c.resolveCloneURL(ctx, job.GitURL, job.IntegrationID)
	if err != nil {
		return fmt.Errorf("resolve clone url: %w", err)
	}

	refs, err := git.LsRemote(cloneURL)
	if err != nil {
		return fmt.Errorf("ls-remote: %w", err)
	}

	desired := planVersions(refs)
	if len(desired) == 0 {
		log.Info("no buildable refs")
		return nil
	}

	for _, d := range desired {
		if err := c.buildVersion(ctx, job, cloneURL, d); err != nil {
			log.Error("build version", "version", d.Version, "err", err)
		}
	}
	return nil
}

type desiredVersion struct {
	Version string
	Ref     git.Ref
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

func (c *Consumer) buildVersion(ctx context.Context, job models.SyncJob, cloneURL string, d desiredVersion) error {
	refType := models.RefTypeTag
	if d.Ref.Type == git.RefBranch {
		refType = models.RefTypeBranch
	}

	existing, err := c.versionRepo.Get(job.AddonID, d.Version)
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
			if rc, err := c.storage.Get(ctx, existing.StorageKey); err == nil {
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
	if err := c.versionRepo.Upsert(av); err != nil {
		return fmt.Errorf("mark building: %w", err)
	}

	rootDir := moduleDir(job.Name, job.Subpath)
	archive, err := git.CloneAndZip(cloneURL, d.Ref.Name, rootDir, job.Subpath)
	if err != nil {
		_ = c.versionRepo.UpdateStatus(av.ID, models.StatusFailed, auth.RedactURLCredentials(err.Error()))
		return err
	}
	defer os.Remove(archive.ZipPath)

	if refType == models.RefTypeTag && archive.Manifest != nil {
		mv := strings.TrimSpace(archive.Manifest.Version)
		mv = strings.TrimPrefix(mv, "v")
		mv = strings.TrimPrefix(mv, "V")
		if mv != "" && mv != d.Version {
			msg := fmt.Sprintf("git tag %s does not match manifest version %s", d.Ref.Name, archive.Manifest.Version)
			_ = c.versionRepo.UpdateStatus(av.ID, models.StatusFailed, msg)
			return fmt.Errorf("%s", msg)
		}
	}

	f, err := os.Open(archive.ZipPath)
	if err != nil {
		_ = c.versionRepo.UpdateStatus(av.ID, models.StatusFailed, auth.RedactURLCredentials(err.Error()))
		return err
	}
	defer f.Close()

	keyName := strings.ReplaceAll(d.Version+"-"+d.Ref.SHA, "/", "-")
	key := fmt.Sprintf("packages/%s/%s.zip", job.AddonID, keyName)
	if err := c.storage.Put(ctx, key, f, storage.PutOptions{ContentType: "application/zip"}); err != nil {
		_ = c.versionRepo.UpdateStatus(av.ID, models.StatusFailed, auth.RedactURLCredentials(err.Error()))
		return err
	}

	if err := c.versionRepo.SetReady(av.ID, key, "sha256:"+archive.ContentHash, archive.SizeBytes); err != nil {
		return fmt.Errorf("mark ready: %w", err)
	}

	if archive.Manifest != nil {
		if err := c.versionRepo.SetManifestMeta(av.ID, archive.Manifest.Depends, archive.Manifest.Version); err != nil {
			slog.Warn("store manifest meta", "addon", job.Name, "version", d.Version, "err", err)
		}
	}

	html, err := markdown.Render(archive.Readme)
	if err != nil {
		slog.Warn("render readme", "addon", job.Name, "version", d.Version, "err", err)
	} else if err := c.versionRepo.SetReadme(av.ID, html); err != nil {
		slog.Warn("store readme", "addon", job.Name, "version", d.Version, "err", err)
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
