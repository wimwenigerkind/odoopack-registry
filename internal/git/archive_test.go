package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testManifest = "{'name': 'Test Addon', 'version': '17.0.1.0.0', 'depends': ['base']}\n"

// commitRepo creates a repository whose single commit holds files, where a
// value starting with "->" becomes a symlink to the rest of the value.
func commitRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if target, ok := strings.CutPrefix(content, "->"); ok {
			if err := os.Symlink(target, path); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"add", "-A"},
		{"-c", "user.name=test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false",
			"commit", "-q", "-m", "init"},
	} {
		if err := runGit(dir, args...); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestReadMetadataSkipsSymlinks(t *testing.T) {
	secret := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(secret, []byte("DATABASE_URL=postgres://leak"), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := commitRepo(t, map[string]string{
		"README.md":       "->" + secret,
		"__manifest__.py": "->" + secret,
	})

	if got := readReadme(dir, "main", ""); got != "" {
		t.Errorf("readme = %q, want empty", got)
	}
	if got := readManifest(dir, "main", ""); got != nil {
		t.Errorf("manifest = %+v, want nil", got)
	}
}

func TestReadReadme(t *testing.T) {
	tests := []struct {
		name    string
		files   map[string]string
		subpath string
		want    string
	}{
		{
			name:  "regular",
			files: map[string]string{"README.md": "# Hello\n"},
			want:  "# Hello\n",
		},
		{
			name:  "lowercase",
			files: map[string]string{"readme.md": "# lower\n"},
			want:  "# lower\n",
		},
		{
			name:  "oversized",
			files: map[string]string{"README.md": strings.Repeat("a", maxMetadataBytes+1)},
			want:  "",
		},
		{
			name: "subpath",
			files: map[string]string{
				"README.md":       "# root\n",
				"addon/README.md": "# addon\n",
			},
			subpath: "/addon/",
			want:    "# addon\n",
		},
		{
			name:  "missing",
			files: map[string]string{"other.txt": "x"},
			want:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := commitRepo(t, tt.files)
			if got := readReadme(dir, "main", tt.subpath); got != tt.want {
				t.Errorf("readme = %.40q, want %.40q", got, tt.want)
			}
		})
	}
}

func TestReadManifest(t *testing.T) {
	dir := commitRepo(t, map[string]string{"addon/__manifest__.py": testManifest})

	m := readManifest(dir, "main", "addon")
	if m == nil {
		t.Fatal("manifest = nil, want parsed manifest")
	}
	if m.Name != "Test Addon" {
		t.Errorf("name = %q, want %q", m.Name, "Test Addon")
	}
}
