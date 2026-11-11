//go:build integration

package handler

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/database"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
	"github.com/wimwenigerkind/odoopack-spec/name"
	"github.com/wimwenigerkind/odoopack-spec/registryapi"
	"gorm.io/gorm"
)

func depsTestDB(t *testing.T) *gorm.DB {
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
	return db
}

type depsFixture struct {
	owner, member, stranger, admin uuid.UUID
	versions                       []models.AddonVersion
	modules                        map[string]string
}

// seedDeps creates addons under a random vendor, so that the module names the
// fixture depends on are its own however many rows the shared database holds.
func seedDeps(t *testing.T, db *gorm.DB) depsFixture {
	t.Helper()
	vendor := "acme" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	create := func(v any) {
		t.Helper()
		if err := db.Create(v).Error; err != nil {
			t.Fatal(err)
		}
	}
	users := make([]models.User, 4)
	for i := range users {
		users[i] = models.User{Username: fmt.Sprintf("%s-%d", vendor, i), IsAdmin: i == 3}
		create(&users[i])
	}
	f := depsFixture{owner: users[0].ID, member: users[1].ID, stranger: users[2].ID, admin: users[3].ID}
	repo := models.Repo{GitURL: "https://git.example.com/" + vendor + ".git", OwnerID: f.owner}
	create(&repo)
	group := models.Group{Name: vendor}
	create(&group)
	create(&models.GroupMembership{GroupID: group.ID, UserID: f.member})

	f.modules = make(map[string]string)
	for _, a := range []struct {
		key        string
		name       string
		visibility models.Visibility
		granted    bool
	}{
		{"public", vendor + "/public-addon", models.VisibilityPublic, false},
		{"private", vendor + "/private", models.VisibilityPrivate, false},
		{"granted", vendor + "/granted", models.VisibilityPrivate, true},
		{"plain", vendor + "_plain", models.VisibilityPublic, false},
	} {
		addon := models.Addon{Name: a.name, RepoID: repo.ID, Visibility: a.visibility}
		create(&addon)
		if a.granted {
			create(&models.GroupAddonAccess{GroupID: group.ID, AddonID: addon.ID})
		}
		f.modules[a.key] = name.ModuleName(a.name)
	}
	f.modules["unknown"] = vendor + "_unknown"

	f.versions = []models.AddonVersion{
		{Version: "19.0.1.0.0", Depends: []string{"base", f.modules["public"], f.modules["private"], f.modules["unknown"]}},
		{Version: "dev-main", Depends: []string{f.modules["granted"], f.modules["plain"], f.modules["public"]}},
		{Version: "18.0.1.0.0"},
	}
	return f
}

// oldResolve is the resolution before REG-14, kept as the oracle: every
// visible addon and every addon name, whatever the versions depend on.
func oldResolve(t *testing.T, db *gorm.DB, versions []models.AddonVersion, userID *uuid.UUID, isAdmin bool, mode string) {
	t.Helper()
	var visible []models.Addon
	if err := repository.NewAddonRepository(db).VisibleQuery(userID, isAdmin, mode, repository.AddonSearch{}).Find(&visible).Error; err != nil {
		t.Fatal(err)
	}
	var allNames []string
	if err := db.Model(&models.Addon{}).Pluck("name", &allNames).Error; err != nil {
		t.Fatal(err)
	}
	resolveDepends(versions, visible, allNames)
}

func TestResolveVersionDependsMatchesFullScan(t *testing.T) {
	db := depsTestDB(t)
	f := seedDeps(t, db)
	addons := repository.NewAddonRepository(db)

	for _, tc := range []struct {
		name    string
		userID  *uuid.UUID
		isAdmin bool
		mode    string
		want    map[string]string
	}{
		{name: "anonymous public", mode: "public", want: map[string]string{
			"public": registryapi.AccessOK, "private": registryapi.AccessForbidden, "unknown": registryapi.AccessExternal}},
		{name: "anonymous private", mode: "private", want: map[string]string{
			"public": registryapi.AccessForbidden, "plain": registryapi.AccessForbidden}},
		{name: "owner private", userID: &f.owner, mode: "private", want: map[string]string{
			"private": registryapi.AccessOK, "granted": registryapi.AccessOK}},
		{name: "group member", userID: &f.member, mode: "public", want: map[string]string{
			"granted": registryapi.AccessOK, "private": registryapi.AccessForbidden}},
		{name: "stranger", userID: &f.stranger, mode: "private", want: map[string]string{
			"public": registryapi.AccessOK, "granted": registryapi.AccessForbidden}},
		{name: "admin", userID: &f.admin, isAdmin: true, mode: "private", want: map[string]string{
			"private": registryapi.AccessOK, "granted": registryapi.AccessOK, "unknown": registryapi.AccessExternal}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := cloneVersions(f.versions)
			resolveVersionDepends(addons, got, tc.userID, tc.isAdmin, tc.mode)
			want := cloneVersions(f.versions)
			oldResolve(t, db, want, tc.userID, tc.isAdmin, tc.mode)

			for i := range want {
				if !reflect.DeepEqual(got[i].DependsResolved, want[i].DependsResolved) {
					t.Errorf("version %s: got %+v, want %+v", want[i].Version, got[i].DependsResolved, want[i].DependsResolved)
				}
			}
			access := make(map[string]string)
			for _, v := range got {
				for _, rd := range v.DependsResolved {
					access[rd.Module] = rd.Access
				}
			}
			for key, wantAccess := range tc.want {
				if got := access[f.modules[key]]; got != wantAccess {
					t.Errorf("%s: access = %q, want %q", key, got, wantAccess)
				}
			}
		})
	}
}

func TestResolveVersionDependsCostIgnoresRegistrySize(t *testing.T) {
	db := depsTestDB(t)
	f := seedDeps(t, db)

	var counting bool
	var queries, rows int64
	err := db.Callback().Query().After("gorm:query").Register("test:count", func(tx *gorm.DB) {
		if counting {
			queries++
			rows += tx.Statement.RowsAffected
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	addons := repository.NewAddonRepository(db)
	measure := func() (int64, int64) {
		queries, rows, counting = 0, 0, true
		resolveVersionDepends(addons, cloneVersions(f.versions), &f.admin, true, "private")
		counting = false
		return queries, rows
	}

	beforeQueries, beforeRows := measure()
	if beforeQueries == 0 {
		t.Fatal("no queries counted")
	}
	var repo models.Repo
	if err := db.Where("owner_id = ?", f.owner).First(&repo).Error; err != nil {
		t.Fatal(err)
	}
	more := make([]models.Addon, 200)
	for i := range more {
		more[i] = models.Addon{Name: fmt.Sprintf("acme/filler-%s", uuid.NewString()), RepoID: repo.ID}
	}
	if err := db.CreateInBatches(more, 100).Error; err != nil {
		t.Fatal(err)
	}
	afterQueries, afterRows := measure()

	if afterQueries != beforeQueries || afterRows != beforeRows {
		t.Errorf("with %d more addons: %d queries reading %d rows, want %d reading %d",
			len(more), afterQueries, afterRows, beforeQueries, beforeRows)
	}
}

func cloneVersions(versions []models.AddonVersion) []models.AddonVersion {
	return append([]models.AddonVersion(nil), versions...)
}
