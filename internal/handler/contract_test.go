package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pb33f/libopenapi"
	validator "github.com/pb33f/libopenapi-validator"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
)

// The contract test runs without Postgres: the repositories are concrete gorm
// types, so the 200 body comes from registryResponse on loaded models and the
// errors from the real Zipball validation and from problem.

const contractBaseURL = "https://registry.example.com"

func contractValidator(t *testing.T) validator.Validator {
	t.Helper()
	spec, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := libopenapi.NewDocument(spec)
	if err != nil {
		t.Fatal(err)
	}
	v, errs := validator.NewValidator(doc)
	if len(errs) > 0 {
		t.Fatalf("build validator: %v", errs)
	}
	if ok, verrs := v.ValidateDocument(); !ok {
		for _, e := range verrs {
			t.Error(e)
		}
		t.FailNow()
	}
	return v
}

func healthCheckAddon() *models.Addon {
	addon := &models.Addon{
		Name: "acme/health-check",
		Versions: []models.AddonVersion{
			{
				Version:     "19.0.1.0.0",
				Status:      models.StatusReady,
				RefValue:    "6b6c390a387c81ee604b602e49ca6c35646cc619",
				ContentHash: "sha256:ece96e65881389c3ef6538ea26b9d7149e8e0af042c828d17283c6b76c633543",
				Depends:     []string{"base", "acme_essentials", "acme_warranty"},
			},
			{
				Version:     "dev-main",
				Status:      models.StatusReady,
				RefValue:    "e83c5163316f89bfbde7d9ab23ca2e25604af290e83c5163316f89bfbde7d9ab",
				ContentHash: "sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae",
			},
			{Version: "19.0.1.1.0", Status: models.StatusBuilding, RefValue: "0d1f2b3c4a5e6f708192a3b4c5d6e7f809182736"},
		},
	}
	addon.ID = uuid.MustParse("f1740dd9-ca0e-42fd-a63b-b553d3fd3882")
	visible := []models.Addon{{Name: "acme/essentials"}}
	resolveDepends(addon.Versions, visible, []string{"acme/essentials", "acme/warranty"})
	return addon
}

func TestRegistryContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	v := contractValidator(t)

	healthCheck := healthCheckAddon()
	unbuilt := &models.Addon{Name: "essentials"}
	byName := map[string]*models.Addon{healthCheck.Name: healthCheck, unbuilt.Name: unbuilt}
	for _, addon := range byName {
		if err := registryResponse(addon, contractBaseURL).Validate(); err != nil {
			t.Errorf("registryResponse(%q).Validate(): %v", addon.Name, err)
		}
	}

	r := gin.New()
	r.GET("/registry/v1/addons/*name", func(c *gin.Context) {
		addon, ok := byName[c.Param("name")[1:]]
		if !ok {
			problem(c, http.StatusNotFound, codeNotFound, "addon not found")
			return
		}
		c.JSON(http.StatusOK, registryResponse(addon, contractBaseURL))
	})
	// The zipball route is the real handler for the checks it makes before it
	// needs the database; the rest are the problems it answers further on.
	zipball := (&RegistryHandler{}).Zipball
	const valid = "f1740dd9-ca0e-42fd-a63b-b553d3fd3882"
	r.GET("/registry/v1/zipball/:addon_id/:reference", func(c *gin.Context) {
		switch c.Param("addon_id") + "/" + c.Param("reference") {
		case valid + "/6b6c390a387c81ee604b602e49ca6c35646cc619":
			problem(c, http.StatusNotFound, codeNotFound, "reference not found")
		case valid + "/0d1f2b3c4a5e6f708192a3b4c5d6e7f809182736":
			problem(c, http.StatusConflict, codeConflict, "version not ready: building")
		case valid + "/deadbeef":
			problem(c, http.StatusInternalServerError, codeInternalError, "internal error")
		default:
			zipball(c)
		}
	})

	tests := []struct {
		name   string
		path   string
		status int
	}{
		{"addon with vendor and depends", "/registry/v1/addons/acme/health-check", http.StatusOK},
		{"addon without vendor or ready versions", "/registry/v1/addons/essentials", http.StatusOK},
		{"unknown addon", "/registry/v1/addons/acme/missing", http.StatusNotFound},
		{"zipball invalid addon id", "/registry/v1/zipball/not-a-uuid/6b6c390a387c81ee604b602e49ca6c35646cc619", http.StatusBadRequest},
		{"zipball invalid reference", "/registry/v1/zipball/" + valid + "/abc", http.StatusBadRequest},
		{"zipball unknown reference", "/registry/v1/zipball/" + valid + "/6b6c390a387c81ee604b602e49ca6c35646cc619", http.StatusNotFound},
		{"zipball version not ready", "/registry/v1/zipball/" + valid + "/0d1f2b3c4a5e6f708192a3b4c5d6e7f809182736", http.StatusConflict},
		{"zipball internal error", "/registry/v1/zipball/" + valid + "/deadbeef", http.StatusInternalServerError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, contractBaseURL+tc.path, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.status, rec.Body)
			}
			if ok, errs := v.ValidateHttpResponse(req, rec.Result()); !ok {
				for _, e := range errs {
					t.Errorf("%s\n%v", e, e.SchemaValidationErrors)
				}
				t.Logf("body: %s", rec.Body)
			}
		})
	}
}

func TestRegistryResponseLeavesOutVersionsOutsideTheContract(t *testing.T) {
	addon := healthCheckAddon()
	legacy := addon.Versions[0]
	legacy.Version = "1.2.3"
	addon.Versions = append(addon.Versions, legacy)

	resp := registryResponse(addon, contractBaseURL)
	if err := resp.Validate(); err != nil {
		t.Fatalf("response breaks the contract: %v", err)
	}
	for _, v := range resp.Versions {
		if v.Version == "1.2.3" {
			t.Fatalf("version %q is outside the contract but was listed", v.Version)
		}
	}
	if len(resp.Versions) != 2 {
		t.Fatalf("got %d versions, want the 2 valid ready ones", len(resp.Versions))
	}
}
