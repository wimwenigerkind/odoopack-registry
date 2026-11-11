//go:build integration

package worker

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/database"
	"github.com/wimwenigerkind/odoopack-registry/internal/git"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
	"gorm.io/gorm"
)

// testDB needs a disposable Postgres database in TEST_DATABASE_DSN; the tests
// empty sync_jobs and addon_versions there. Packages with integration tests
// share that database, so run them one at a time: go test -p 1 -tags integration.
func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN not set")
	}
	db, err := database.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db, nil); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("TRUNCATE sync_jobs, addon_versions").Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func newAddon(t *testing.T, db *gorm.DB) models.Addon {
	t.Helper()
	user := models.User{Username: "acme"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	repo := models.Repo{GitURL: "https://git.example.com/acme/" + uuid.NewString() + ".git", OwnerID: user.ID}
	if err := db.Create(&repo).Error; err != nil {
		t.Fatal(err)
	}
	addon := models.Addon{Name: "acme_" + strings.ReplaceAll(uuid.NewString(), "-", ""), RepoID: repo.ID, Repo: &repo}
	if err := db.Omit("Repo").Create(&addon).Error; err != nil {
		t.Fatal(err)
	}
	return addon
}

func enqueue(t *testing.T, q *Queue, addon models.Addon) {
	t.Helper()
	if err := q.Enqueue(SyncJob{AddonID: addon.ID, Name: addon.Name, GitURL: addon.Repo.GitURL, Trigger: "test"}); err != nil {
		t.Fatal(err)
	}
	makeDue(t, q.db, addon.ID)
}

// makeDue sets run_after by the database clock, which the claim compares
// against and which may lag behind the clock of the test.
func makeDue(t *testing.T, db *gorm.DB, addonID uuid.UUID) {
	t.Helper()
	if err := db.Model(&models.SyncJob{}).Where("addon_id = ?", addonID).Update("run_after", gorm.Expr("now()")).Error; err != nil {
		t.Fatal(err)
	}
}

func countJobs(t *testing.T, db *gorm.DB, addonID uuid.UUID, status models.SyncJobStatus) int64 {
	t.Helper()
	var n int64
	if err := db.Model(&models.SyncJob{}).Where("addon_id = ? AND status = ?", addonID, status).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func newTestConsumer(db *gorm.DB) *Consumer {
	return NewConsumer(db, repository.NewAddonVersionRepository(db), nil, nil, nil, time.Second)
}

func TestEnqueueKeepsOnePendingJob(t *testing.T) {
	db := testDB(t)
	q := NewQueue(db)
	addon := newAddon(t, db)

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if err := q.Enqueue(SyncJob{AddonID: addon.ID, Name: addon.Name, Trigger: "test"}); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if n := countJobs(t, db, addon.ID, models.SyncJobPending); n != 1 {
		t.Fatalf("pending jobs = %d, want 1", n)
	}

	dup := models.SyncJob{AddonID: addon.ID, Status: models.SyncJobPending, RunAfter: time.Now()}
	if err := db.Create(&dup).Error; !errors.Is(err, gorm.ErrDuplicatedKey) {
		t.Errorf("second pending job: err = %v, want ErrDuplicatedKey", err)
	}
}

func TestClaimTakesOneJobPerAddon(t *testing.T) {
	db := testDB(t)
	q := NewQueue(db)
	addon := newAddon(t, db)
	enqueue(t, q, addon)

	workers := []*Consumer{newTestConsumer(db), newTestConsumer(db)}
	claimed := make([]bool, len(workers))
	var wg sync.WaitGroup
	for i, c := range workers {
		wg.Go(func() {
			_, ok, err := c.claim(context.Background())
			if err != nil {
				t.Error(err)
			}
			claimed[i] = ok
		})
	}
	wg.Wait()
	if claimed[0] == claimed[1] {
		t.Fatalf("claimed = %v, want exactly one", claimed)
	}

	// A push while the job runs queues another one, which waits for it.
	enqueue(t, q, addon)
	if n := countJobs(t, db, addon.ID, models.SyncJobPending); n != 1 {
		t.Fatalf("pending jobs = %d, want 1", n)
	}
	for _, c := range workers {
		if _, ok, err := c.claim(context.Background()); err != nil || ok {
			t.Errorf("claim with a running job = %v, %v, want nothing", ok, err)
		}
	}

	other := newAddon(t, db)
	enqueue(t, q, other)
	job, ok, err := workers[0].claim(context.Background())
	if err != nil || !ok || job.AddonID != other.ID {
		t.Errorf("claim = %v, %v, %v, want the job of the other addon", job.AddonID, ok, err)
	}
}

func TestReapRequeuesStaleJob(t *testing.T) {
	db := testDB(t)
	stale := newAddon(t, db)
	superseded := newAddon(t, db)
	live := newAddon(t, db)

	old := time.Now().Add(-staleAfter - time.Minute)
	recent := time.Now()
	for _, j := range []models.SyncJob{
		{AddonID: stale.ID, Status: models.SyncJobRunning, LockedAt: &old, LockedBy: "worker-gone", MaxAttempts: 3},
		{AddonID: superseded.ID, Status: models.SyncJobRunning, LockedAt: &old, LockedBy: "worker-gone", MaxAttempts: 3},
		{AddonID: superseded.ID, Status: models.SyncJobPending, MaxAttempts: 3},
		{AddonID: live.ID, Status: models.SyncJobRunning, LockedAt: &recent, LockedBy: "worker-live", MaxAttempts: 3},
	} {
		j.RunAfter = time.Now()
		if err := db.Create(&j).Error; err != nil {
			t.Fatal(err)
		}
	}
	versionRepo := repository.NewAddonVersionRepository(db)
	const sha, rebuildSHA = "0123456789abcdef0123456789abcdef01234567", "89abcdef0123456789abcdef0123456789abcdef"
	for _, v := range []models.AddonVersion{
		{AddonID: stale.ID, Version: "dev-main", Status: models.StatusBuilding, BuildingRef: sha},
		{AddonID: stale.ID, Version: "dev-legacy", Status: models.StatusBuilding},
		{AddonID: stale.ID, Version: "dev-rebuild", Status: models.StatusReady, ContentHash: "sha256:old", BuildingRef: rebuildSHA},
		{AddonID: stale.ID, Version: "19.0.1.0.0", Status: models.StatusReady},
		{AddonID: superseded.ID, Version: "dev-main", Status: models.StatusBuilding, BuildingRef: sha},
		{AddonID: live.ID, Version: "dev-main", Status: models.StatusBuilding, BuildingRef: sha},
		{AddonID: live.ID, Version: "dev-rebuild", Status: models.StatusReady, BuildingRef: rebuildSHA},
	} {
		v.RefType, v.RefValue = models.RefTypeBranch, sha
		if err := db.Create(&v).Error; err != nil {
			t.Fatal(err)
		}
	}

	jobs, versions, err := reap(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if jobs != 2 || versions != 4 {
		t.Errorf("reaped %d jobs and %d versions, want 2 and 4", jobs, versions)
	}

	var job models.SyncJob
	if err := db.Where("addon_id = ?", stale.ID).First(&job).Error; err != nil {
		t.Fatal(err)
	}
	if job.Status != models.SyncJobPending || job.Attempts != 1 || job.LockedAt != nil || job.LockedBy != "" || !job.RunAfter.After(time.Now()) {
		t.Errorf("stale job = %+v, want pending with one attempt, unlocked, backed off", job)
	}
	if n := countJobs(t, db, superseded.ID, models.SyncJobRunning) + countJobs(t, db, superseded.ID, models.SyncJobPending); n != 1 {
		t.Errorf("superseded addon has %d jobs, want only its pending one", n)
	}
	if n := countJobs(t, db, live.ID, models.SyncJobRunning); n != 1 {
		t.Errorf("live addon has %d running jobs, want 1", n)
	}

	// A hung rebuild of a ready version keeps the good build and only records
	// the error.
	for _, tc := range []struct {
		addon       uuid.UUID
		version     string
		status      models.VersionStatus
		buildingRef string
		buildErr    string
	}{
		{stale.ID, "dev-main", models.StatusFailed, "", staleBuildErr},
		{stale.ID, "dev-legacy", models.StatusFailed, "", staleBuildErr},
		{stale.ID, "dev-rebuild", models.StatusReady, "", staleBuildErr},
		{stale.ID, "19.0.1.0.0", models.StatusReady, "", ""},
		{superseded.ID, "dev-main", models.StatusFailed, "", staleBuildErr},
		{live.ID, "dev-main", models.StatusBuilding, sha, ""},
		{live.ID, "dev-rebuild", models.StatusReady, rebuildSHA, ""},
	} {
		v, err := versionRepo.Get(tc.addon, tc.version)
		if err != nil {
			t.Fatal(err)
		}
		if v.Status != tc.status || v.BuildingRef != tc.buildingRef || v.BuildError != tc.buildErr {
			t.Errorf("%s = %s building %q error %q, want %s building %q error %q",
				tc.version, v.Status, v.BuildingRef, v.BuildError, tc.status, tc.buildingRef, tc.buildErr)
		}
		if tc.version == "dev-rebuild" && (v.RefValue != sha || (tc.addon == stale.ID && v.ContentHash != "sha256:old")) {
			t.Errorf("%s moved to %s %s, want the good build", tc.version, v.RefValue, v.ContentHash)
		}
	}

	if jobs, _, err := reap(context.Background(), db); err != nil || jobs != 0 {
		t.Errorf("second reap = %d, %v, want nothing", jobs, err)
	}
}

func TestFailedBuildIsRetriedUntilMaxAttempts(t *testing.T) {
	db := testDB(t)
	addon := newAddon(t, db)
	enqueue(t, NewQueue(db), addon)

	c := newTestConsumer(db)
	c.lsRemote = func(context.Context, string, []string) ([]git.Ref, error) {
		return []git.Ref{{Type: git.RefTag, Name: "19.0.1.0.0", SHA: "0123456789abcdef0123456789abcdef01234567"}}, nil
	}
	cloneErr := errors.New("git clone https://oauth2:s3cret@git.example.com/acme/addons.git: unreachable")
	c.cloneAndZip = func(context.Context, string, []string, string, string, string) (git.ArchiveResult, error) {
		return git.ArchiveResult{}, cloneErr
	}
	log := slog.New(slog.DiscardHandler)

	for attempt := 1; attempt <= 3; attempt++ {
		makeDue(t, db, addon.ID)
		job, ok, err := c.claim(context.Background())
		if err != nil || !ok {
			t.Fatalf("attempt %d: claim = %v, %v", attempt, ok, err)
		}
		if err := c.process(context.Background(), log, job); !errors.Is(err, cloneErr) {
			t.Fatalf("process err = %v, want the clone error", err)
		}
		c.handle(context.Background(), log, job)

		var got models.SyncJob
		if err := db.Where("id = ?", job.ID).First(&got).Error; err != nil {
			t.Fatal(err)
		}
		want := models.SyncJobPending
		if attempt == 3 {
			want = models.SyncJobFailed
		}
		if got.Status != want || got.Attempts != attempt {
			t.Errorf("attempt %d: job %s with %d attempts, want %s", attempt, got.Status, got.Attempts, want)
		}
		if strings.Contains(got.LastError, "s3cret") || !strings.Contains(got.LastError, "unreachable") {
			t.Errorf("last_error = %q, want the redacted clone error", got.LastError)
		}
	}

	v, err := repository.NewAddonVersionRepository(db).Get(addon.ID, "19.0.1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if v.Status != models.StatusFailed || strings.Contains(v.BuildError, "s3cret") {
		t.Errorf("version = %s %q, want failed with a redacted error", v.Status, v.BuildError)
	}

	// A failed job no longer blocks the addon.
	enqueue(t, NewQueue(db), addon)
	if n := countJobs(t, db, addon.ID, models.SyncJobPending); n != 1 {
		t.Errorf("pending jobs after failure = %d, want 1", n)
	}
}

// A job that fails while a push queued another one gives way to it instead of
// colliding with it on the unique index.
func TestFailedJobGivesWayToPendingJob(t *testing.T) {
	db := testDB(t)
	addon := newAddon(t, db)
	q := NewQueue(db)
	enqueue(t, q, addon)

	c := newTestConsumer(db)
	c.lsRemote = func(context.Context, string, []string) ([]git.Ref, error) {
		return nil, errors.New("unreachable")
	}
	job, ok, err := c.claim(context.Background())
	if err != nil || !ok {
		t.Fatalf("claim = %v, %v", ok, err)
	}
	enqueue(t, q, addon)
	c.handle(context.Background(), slog.New(slog.DiscardHandler), job)

	if err := db.Where("id = ?", job.ID).First(&models.SyncJob{}).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("failed job: err = %v, want it deleted", err)
	}
	if n := countJobs(t, db, addon.ID, models.SyncJobPending); n != 1 {
		t.Errorf("pending jobs = %d, want 1", n)
	}
}
