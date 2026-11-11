package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"gorm.io/gorm"
)

// A job may stay running for the job timeout plus staleMargin before the
// reaper assumes its worker died, so a live worker always finishes or fails
// its job before the reaper touches it.
const staleMargin = 5 * time.Minute

const reapInterval = time.Minute

const (
	staleJobError = "worker stopped while the job was running"
	staleBuildErr = "build interrupted: the worker stopped before it finished"
)

func (c *Consumer) reapLoop(ctx context.Context) {
	defer c.wg.Done()
	t := time.NewTicker(reapInterval)
	defer t.Stop()
	for {
		jobs, versions, err := reap(ctx, c.db, c.limits.JobTimeout+staleMargin)
		switch {
		case err != nil:
			if ctx.Err() == nil {
				slog.Error("reap stale jobs", "err", err)
			}
		case jobs > 0:
			slog.Warn("reaped stale jobs", "jobs", jobs, "failed_versions", versions)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// reap returns jobs whose worker died to pending and fails the builds those
// workers left unfinished: a version in building goes to failed, a ready one
// keeps its last good build and only records the error. It is one statement
// whose stale rows are locked with SKIP LOCKED, so replicas reaping at the
// same time never handle a job twice. A reaped job counts as an attempt, so
// one that keeps killing its worker ends as failed. When a newer pending job
// exists for the addon, the stale one is deleted instead, since only one
// pending job per addon may exist.
func reap(ctx context.Context, db *gorm.DB, staleAfter time.Duration) (jobs, versions int64, err error) {
	err = db.WithContext(ctx).Raw(`
WITH stale AS (
	SELECT id FROM sync_jobs
	WHERE status = @running AND locked_at < now() - make_interval(secs => @stale_after)
	FOR UPDATE SKIP LOCKED
),
requeued AS (
	UPDATE sync_jobs j SET
		status = CASE WHEN j.attempts + 1 >= j.max_attempts THEN @failed ELSE @pending END,
		attempts = j.attempts + 1,
		run_after = now() + make_interval(secs => @backoff * (j.attempts + 1)),
		last_error = @job_error,
		locked_at = NULL,
		locked_by = '',
		updated_at = now()
	FROM stale s
	WHERE j.id = s.id
	AND NOT EXISTS (SELECT 1 FROM sync_jobs p WHERE p.addon_id = j.addon_id AND p.status = @pending)
	RETURNING j.addon_id
),
dropped AS (
	DELETE FROM sync_jobs j USING stale s
	WHERE j.id = s.id
	AND EXISTS (SELECT 1 FROM sync_jobs p WHERE p.addon_id = j.addon_id AND p.status = @pending)
	RETURNING j.addon_id
),
failed AS (
	UPDATE addon_versions SET
		status = CASE WHEN status = @building THEN @version_failed ELSE status END,
		build_error = @build_error,
		building_ref = '',
		updated_at = now()
	WHERE (status = @building OR building_ref <> '')
	AND addon_id IN (SELECT addon_id FROM requeued UNION ALL SELECT addon_id FROM dropped)
	RETURNING 1
)
SELECT (SELECT count(*) FROM requeued) + (SELECT count(*) FROM dropped), (SELECT count(*) FROM failed)`,
		map[string]any{
			"running":        models.SyncJobRunning,
			"pending":        models.SyncJobPending,
			"failed":         models.SyncJobFailed,
			"stale_after":    staleAfter.Seconds(),
			"backoff":        retryBackoff.Seconds(),
			"job_error":      staleJobError,
			"version_failed": models.StatusFailed,
			"building":       models.StatusBuilding,
			"build_error":    staleBuildErr,
		}).Row().Scan(&jobs, &versions)
	return jobs, versions, err
}
