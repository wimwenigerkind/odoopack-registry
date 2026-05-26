package worker

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/google/uuid"
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
}

type Queue struct {
	jobs        chan SyncJob
	versionRepo *repository.AddonVersionRepository
	storage     storage.Storage
	wg          sync.WaitGroup
}

func NewQueue(versionRepo *repository.AddonVersionRepository, store storage.Storage, bufferSize int) *Queue {
	return &Queue{
		jobs:        make(chan SyncJob, bufferSize),
		versionRepo: versionRepo,
		storage:     store,
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
	log.Printf("worker %d: sync %s from %s (trigger=%s)", workerID, job.Name, job.GitURL, job.Trigger)

	refs, err := git.LsRemote(job.GitURL)
	if err != nil {
		log.Printf("worker %d: ls-remote %s: %v", workerID, job.Name, err)
		return
	}

	desired := planVersions(refs, job.DefaultBranch)
	if len(desired) == 0 {
		log.Printf("worker %d: %s has no buildable refs", workerID, job.Name)
		return
	}

	for _, d := range desired {
		if err := q.buildVersion(ctx, job, d); err != nil {
			log.Printf("worker %d: build %s@%s: %v", workerID, job.Name, d.Version, err)
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

func (q *Queue) buildVersion(ctx context.Context, job SyncJob, d desiredVersion) error {
	refType := models.RefTypeTag
	if d.Ref.Type == git.RefBranch {
		refType = models.RefTypeBranch
	}

	existing, err := q.versionRepo.Get(job.AddonID, d.Version)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return fmt.Errorf("lookup existing: %w", err)
	}
	if existing != nil && existing.Status == models.StatusReady && existing.RefValue == d.Ref.SHA {
		return nil
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
	archive, err := git.CloneAndZip(job.GitURL, d.Ref.Name, rootDir)
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
	log.Printf("worker: built %s@%s (%d bytes)", job.Name, d.Version, archive.SizeBytes)
	return nil
}

func sanitize(s string) string {
	return strings.ReplaceAll(s, "/", "_")
}
