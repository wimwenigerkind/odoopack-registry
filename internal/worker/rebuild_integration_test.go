//go:build integration

package worker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	odoomanifest "github.com/wimwenigerkind/odoo-manifest"
	"github.com/wimwenigerkind/odoopack-registry/internal/auth"
	"github.com/wimwenigerkind/odoopack-registry/internal/git"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
	"github.com/wimwenigerkind/odoopack-registry/internal/storage"
	"gorm.io/gorm"
)

const (
	shaOld = "1111111111111111111111111111111111111111"
	shaNew = "2222222222222222222222222222222222222222"
)

// buildConsumer builds into local storage; build decides what the clone of a
// commit yields and may inspect the database while that build runs.
func buildConsumer(t *testing.T, db *gorm.DB, build func(sha string) (git.ArchiveResult, error)) *Consumer {
	t.Helper()
	store, err := storage.NewLocalStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c := NewConsumer(db, repository.NewAddonVersionRepository(db), nil, nil, store, time.Second)
	c.cloneAndZip = func(_ context.Context, _ string, _ []string, ref, _, _ string) (git.ArchiveResult, error) {
		return build(ref)
	}
	return c
}

func archive(t *testing.T, hash string, depends ...string) git.ArchiveResult {
	t.Helper()
	path := filepath.Join(t.TempDir(), hash+".zip")
	if err := os.WriteFile(path, []byte(hash), 0o600); err != nil {
		t.Fatal(err)
	}
	return git.ArchiveResult{ZipPath: path, ContentHash: hash, SizeBytes: int64(len(hash)),
		Manifest: &odoomanifest.Manifest{Depends: depends}}
}

func hashOf(sha string) string {
	return "aa" + sha + sha[:22]
}

// buildAt runs buildVersion for dev-main at sha. The fake clone gets the sha
// as its ref, so a test can tell the builds apart.
func buildAt(c *Consumer, addon models.Addon, sha string) error {
	job := models.SyncJob{AddonID: addon.ID, Name: addon.Name}
	d := desiredVersion{Version: "dev-main", Ref: git.Ref{Type: git.RefBranch, Name: sha, SHA: sha}}
	return c.buildVersion(context.Background(), job, auth.GitRemote{}, d)
}

// listed returns dev-main as the registry metadata sees it: the addon loaded
// by the query the registry handler uses, reduced to its ready versions.
func listed(t *testing.T, db *gorm.DB, addon models.Addon) *models.AddonVersion {
	t.Helper()
	a, err := repository.NewAddonRepository(db).GetByName(addon.Name)
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range a.Versions {
		if v.Version == "dev-main" && v.Status == models.StatusReady {
			return &a.Versions[i]
		}
	}
	return nil
}

func wantListedAt(t *testing.T, db *gorm.DB, addon models.Addon, sha string, depends ...string) *models.AddonVersion {
	t.Helper()
	v := listed(t, db, addon)
	if v == nil {
		t.Fatal("dev-main is not listed")
	}
	if v.RefValue != sha || v.ContentHash != "sha256:"+hashOf(sha) || !slices.Equal(v.Depends, depends) {
		t.Fatalf("dev-main listed at %s %s %v, want %s %s %v", v.RefValue, v.ContentHash, v.Depends, sha, "sha256:"+hashOf(sha), depends)
	}
	return v
}

func TestFailedRebuildKeepsLastGoodBuild(t *testing.T) {
	db := testDB(t)
	addon := newAddon(t, db)

	var during *models.AddonVersion
	c := buildConsumer(t, db, func(sha string) (git.ArchiveResult, error) {
		if sha == shaOld {
			return archive(t, hashOf(sha), "base"), nil
		}
		during = wantListedAt(t, db, addon, shaOld, "base")
		return git.ArchiveResult{}, errors.New("git clone: unreachable")
	})
	if err := buildAt(c, addon, shaOld); err != nil {
		t.Fatal(err)
	}
	good := wantListedAt(t, db, addon, shaOld, "base")

	if err := buildAt(c, addon, shaNew); err == nil {
		t.Fatal("rebuild succeeded, want the clone error")
	}
	if during == nil || during.BuildingRef != shaNew {
		t.Errorf("during the rebuild building_ref = %+v, want %s", during, shaNew)
	}
	after := wantListedAt(t, db, addon, shaOld, "base")
	if after.ID != good.ID || after.StorageKey != good.StorageKey || after.BuildingRef != "" || after.BuildError != "git clone: unreachable" {
		t.Errorf("after the failed rebuild = %+v, want the good build with the rebuild error", after)
	}
	if rc, err := c.storage.Get(context.Background(), after.StorageKey); err != nil {
		t.Errorf("artifact of the good build: %v", err)
	} else {
		_ = rc.Close()
	}
}

func TestRebuildSwapsToNewBuild(t *testing.T) {
	db := testDB(t)
	addon := newAddon(t, db)
	c := buildConsumer(t, db, func(sha string) (git.ArchiveResult, error) {
		if sha == shaOld {
			return archive(t, hashOf(sha), "base"), nil
		}
		wantListedAt(t, db, addon, shaOld, "base")
		return archive(t, hashOf(sha), "base", "sale"), nil
	})
	if err := buildAt(c, addon, shaOld); err != nil {
		t.Fatal(err)
	}
	old := wantListedAt(t, db, addon, shaOld, "base")
	// A failed attempt in between must not survive the successful one.
	if err := db.Model(&models.AddonVersion{}).Where("id = ?", old.ID).Update("build_error", "earlier failure").Error; err != nil {
		t.Fatal(err)
	}

	if err := buildAt(c, addon, shaNew); err != nil {
		t.Fatal(err)
	}
	v := wantListedAt(t, db, addon, shaNew, "base", "sale")
	if v.ID != old.ID || v.StorageKey == old.StorageKey || v.SizeBytes != int64(len(hashOf(shaNew))) || v.BuildingRef != "" || v.BuildError != "" {
		t.Errorf("after the rebuild = %+v, want the new build in the same row", v)
	}
	// The previous artifact stays in storage for clients pinned to its hash.
	if rc, err := c.storage.Get(context.Background(), old.StorageKey); err != nil {
		t.Errorf("artifact of the previous build: %v", err)
	} else {
		_ = rc.Close()
	}
}

func TestFirstBuildGoesThroughBuilding(t *testing.T) {
	for _, fail := range []bool{false, true} {
		db := testDB(t)
		addon := newAddon(t, db)
		versions := repository.NewAddonVersionRepository(db)
		var during models.VersionStatus
		c := buildConsumer(t, db, func(sha string) (git.ArchiveResult, error) {
			v, err := versions.Get(addon.ID, "dev-main")
			if err != nil {
				t.Fatal(err)
			}
			during = v.Status
			if fail {
				return git.ArchiveResult{}, errors.New("git clone: unreachable")
			}
			return archive(t, hashOf(sha)), nil
		})
		err := buildAt(c, addon, shaOld)
		if fail != (err != nil) {
			t.Fatalf("fail=%v: build err = %v", fail, err)
		}
		if during != models.StatusBuilding {
			t.Errorf("fail=%v: status during the build = %s, want building", fail, during)
		}
		v, err := versions.Get(addon.ID, "dev-main")
		if err != nil {
			t.Fatal(err)
		}
		want := models.StatusReady
		if fail {
			want = models.StatusFailed
		}
		if v.Status != want || v.RefValue != shaOld || v.BuildingRef != "" {
			t.Errorf("fail=%v: version = %s at %s building %q, want %s at %s", fail, v.Status, v.RefValue, v.BuildingRef, want, shaOld)
		}
		if got := listed(t, db, addon) != nil; got == fail {
			t.Errorf("fail=%v: listed = %v", fail, got)
		}
	}
}
