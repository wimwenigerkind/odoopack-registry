package handler

import (
	"testing"

	"github.com/google/uuid"
)

func TestIntegrationForGitURL(t *testing.T) {
	id := uuid.New()
	cases := []struct {
		name          string
		integrationID *uuid.UUID
		oldURL        string
		newURL        string
		want          *uuid.UUID
	}{
		{"same host", &id, "https://github.com/acme/r.git", "https://github.com/acme/other.git", &id},
		{"same host as scp", &id, "https://github.com/acme/r.git", "git@github.com:acme/r.git", &id},
		{"same host in other case", &id, "https://github.com/acme/r.git", "https://GitHub.com/acme/r.git", &id},
		{"other host", &id, "https://github.com/acme/r.git", "https://git.acme.example/acme/r.git", nil},
		{"other host as scp", &id, "https://github.com/acme/r.git", "git@git.acme.example:acme/r.git", nil},
		{"no integration", nil, "https://github.com/acme/r.git", "https://git.acme.example/acme/r.git", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := integrationForGitURL(tc.integrationID, tc.oldURL, tc.newURL); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
