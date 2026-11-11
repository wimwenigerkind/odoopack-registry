package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
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
	"github.com/wimwenigerkind/odoopack-spec/name"
	"github.com/wimwenigerkind/odoopack-spec/version"
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

	lsRemote    func(ctx context.Context, url string, env []string) ([]git.Ref, error)
	cloneAndZip func(ctx context.Context, url string, env []string, ref, rootDir, subpath string) (git.ArchiveResult, error)
}

// jobTimeout bounds one sync job, git included. The chart's
// worker.terminationGracePeriodSeconds must stay above it, because a
// shutdown lets the running job finish instead of discarding its builds.
const jobTimeout = 10 * time.Minute

// retryBackoff is the delay before a failed job runs again, multiplied by
// the number of attempts so far.
const retryBackoff = 30 * time.Second

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
		lsRemote:        git.LsRemote,
		cloneAndZip:     git.CloneAndZip,
	}
}

func (c *Consumer) Run(ctx context.Context, workers int) {
	c.wg.Add(1)
	go c.reapLoop(ctx)
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
	SELECT id FROM sync_jobs j
	WHERE status = ? AND run_after <= now()
	AND NOT EXISTS (SELECT 1 FROM sync_jobs r WHERE r.addon_id = j.addon_id AND r.status = ?)
	ORDER BY created_at
	FOR UPDATE SKIP LOCKED
	LIMIT 1
)
RETURNING *`, models.SyncJobRunning, c.id, models.SyncJobPending, models.SyncJobRunning).Scan(&job).Error
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
	log.Info("sync start", "git_url", auth.RedactURLCredentials(job.GitURL))

	// A shutdown stops the claiming but not the job in hand: it runs to its
	// own deadline within the pod's termination grace period.
	jobCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), jobTimeout)
	defer cancel()

	if err := c.process(jobCtx, log, job); err != nil {
		attempts := job.Attempts + 1
		errMsg := auth.RedactURLCredentials(err.Error())
		updates := map[string]any{
			"attempts":   attempts,
			"last_error": errMsg,
			"locked_at":  nil,
			"locked_by":  "",
			"updated_at": time.Now(),
		}
		if attempts >= job.MaxAttempts {
			updates["status"] = models.SyncJobFailed
			log.Error("sync failed permanently", "attempts", attempts, "err", errMsg)
			c.db.Model(&models.SyncJob{}).Where("id = ?", job.ID).Updates(updates)
		} else {
			updates["status"] = models.SyncJobPending
			updates["run_after"] = time.Now().Add(time.Duration(attempts) * retryBackoff)
			log.Warn("sync failed, will retry", "attempts", attempts, "err", errMsg)
			c.requeue(log, job, updates)
		}
		return
	}

	c.db.Where("id = ?", job.ID).Delete(&models.SyncJob{})
	log.Info("sync done")
}

// requeue returns a running job to pending. Only one pending job per addon
// may exist, and one that was enqueued meanwhile syncs every ref anyway, so
// the failed job gives way to it.
func (c *Consumer) requeue(log *slog.Logger, job models.SyncJob, updates map[string]any) {
	res := c.db.Model(&models.SyncJob{}).
		Where("id = ? AND NOT EXISTS (SELECT 1 FROM sync_jobs p WHERE p.addon_id = ? AND p.status = ?)",
			job.ID, job.AddonID, models.SyncJobPending).
		Updates(updates)
	if res.Error == nil && res.RowsAffected > 0 {
		return
	}
	if res.Error != nil && !errors.Is(res.Error, gorm.ErrDuplicatedKey) {
		log.Error("requeue job", "err", res.Error)
		return
	}
	if err := c.db.Where("id = ?", job.ID).Delete(&models.SyncJob{}).Error; err != nil {
		log.Error("delete superseded job", "err", err)
		return
	}
	log.Info("sync superseded by a pending job")
}

func (c *Consumer) process(ctx context.Context, log *slog.Logger, job models.SyncJob) error {
	remote, err := c.resolveGitRemote(ctx, job.GitURL, job.IntegrationID)
	if err != nil {
		return fmt.Errorf("resolve git remote: %w", err)
	}

	refs, err := c.lsRemote(ctx, remote.URL, remote.Env)
	if err != nil {
		return fmt.Errorf("ls-remote: %w", err)
	}

	desired := planVersions(refs)
	if len(desired) == 0 {
		log.Info("no buildable refs")
		return nil
	}

	var errs []error
	for _, d := range desired {
		if err := c.buildVersion(ctx, job, remote, d); err != nil {
			log.Error("build version", "version", d.Version, "err", auth.RedactURLCredentials(err.Error()))
			errs = append(errs, fmt.Errorf("build %s: %w", d.Version, err))
		}
	}
	return errors.Join(errs...)
}

type desiredVersion struct {
	Version string
	Ref     git.Ref
}

func planVersions(refs []git.Ref) []desiredVersion {
	var out []desiredVersion
	for _, r := range refs {
		v, err := refVersion(r)
		if err != nil {
			slog.Warn("skipping ref", "type", r.Type, "ref", r.Name, "reason", err)
			continue
		}
		out = append(out, desiredVersion{Version: v, Ref: r})
	}
	return out
}

// refVersion publishes a ref only under the canonical spelling that clients
// use as a key and that the registry API accepts, so a tag like 1.2.3 or
// 19.0.01.0.0 and a branch git allows but the version rules do not are skipped
// instead of becoming a version every client rejects.
func refVersion(r git.Ref) (string, error) {
	var s string
	switch r.Type {
	case git.RefTag:
		s = r.Name
		if s != "" && (s[0] == 'v' || s[0] == 'V') {
			s = s[1:]
		}
	case git.RefBranch:
		s = "dev-" + r.Name
	default:
		return "", fmt.Errorf("unknown ref type %q", r.Type)
	}
	v, err := version.Parse(s)
	if err != nil {
		return "", err
	}
	if v.String() != s {
		return "", fmt.Errorf("version %q is not canonical, want %q", s, v.String())
	}
	// A tag named dev-<x> parses as a dev version but would be published as an immutable tag.
	if r.Type == git.RefTag && !v.IsRelease() {
		return "", fmt.Errorf("tag %q is not a release version", r.Name)
	}
	return s, nil
}

func (c *Consumer) buildVersion(ctx context.Context, job models.SyncJob, remote auth.GitRemote, d desiredVersion) error {
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
				_ = rc.Close()
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
	}
	if err := c.versionRepo.StartBuild(av); err != nil {
		return fmt.Errorf("mark building: %w", err)
	}

	rootDir := name.ModuleName(job.Name)
	archive, err := c.cloneAndZip(ctx, remote.URL, remote.Env, d.Ref.Name, rootDir, job.Subpath)
	if err != nil {
		_ = c.versionRepo.FailBuild(av.ID, auth.RedactURLCredentials(err.Error()))
		return err
	}
	defer func() { _ = os.Remove(archive.ZipPath) }()

	f, err := os.Open(archive.ZipPath)
	if err != nil {
		_ = c.versionRepo.FailBuild(av.ID, auth.RedactURLCredentials(err.Error()))
		return err
	}
	defer func() { _ = f.Close() }()

	keyName := strings.ReplaceAll(d.Version+"-"+d.Ref.SHA, "/", "-")
	key := fmt.Sprintf("packages/%s/%s.zip", job.AddonID, keyName)
	if err := c.storage.Put(ctx, key, f, storage.PutOptions{ContentType: "application/zip"}); err != nil {
		_ = c.versionRepo.FailBuild(av.ID, auth.RedactURLCredentials(err.Error()))
		return err
	}

	build := repository.Build{
		RefValue:    d.Ref.SHA,
		StorageKey:  key,
		ContentHash: "sha256:" + archive.ContentHash,
		SizeBytes:   archive.SizeBytes,
	}
	if m := archive.Manifest; m != nil {
		build.Manifest = &repository.ManifestMeta{
			Depends:         m.Depends,
			ManifestVersion: m.Version,
			Summary:         m.Summary,
			License:         m.License,
			Category:        m.Category,
			Author:          m.Author,
			Website:         m.Website,
		}
	}
	if err := c.versionRepo.SetReady(av.ID, build); err != nil {
		return fmt.Errorf("mark ready: %w", err)
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
