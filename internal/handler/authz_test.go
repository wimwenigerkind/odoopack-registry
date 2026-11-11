package handler

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
)

func addonWithWriterFields() *models.Addon {
	integrationID := uuid.New()
	return &models.Addon{
		Name: "sale_extra",
		Repo: &models.Repo{
			IntegrationID: &integrationID,
			Integration:   &models.OAuthIntegration{Provider: "github", AccountName: "acme"},
		},
		Versions: []models.AddonVersion{
			{Version: "1.0.0", Status: models.StatusFailed, BuildError: "git clone failed"},
			// A failed rebuild leaves the version ready with the rebuild's error.
			{Version: "dev-main", Status: models.StatusReady, BuildError: "rebuild failed"},
		},
	}
}

func TestHideFromNonWriters(t *testing.T) {
	t.Run("non-writer", func(t *testing.T) {
		addon := addonWithWriterFields()
		hideFromNonWriters(addon, false)
		body, err := json.Marshal(addon)
		if err != nil {
			t.Fatal(err)
		}
		for _, field := range []string{"build_error", "integration"} {
			if strings.Contains(string(body), `"`+field) {
				t.Errorf("response contains %q: %s", field, body)
			}
		}
	})
	t.Run("writer", func(t *testing.T) {
		addon := addonWithWriterFields()
		hideFromNonWriters(addon, true)
		body, err := json.Marshal(addon)
		if err != nil {
			t.Fatal(err)
		}
		for _, field := range []string{`"build_error":"git clone failed"`, `"build_error":"rebuild failed"`, `"integration_id"`, `"account_name":"acme"`} {
			if !strings.Contains(string(body), field) {
				t.Errorf("response lacks %s: %s", field, body)
			}
		}
	})
}
