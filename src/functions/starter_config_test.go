package functions

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
	"github.com/spf13/afero"
)

func TestWriteStarterRepoConfig_EmitsExpectedShape(t *testing.T) {
	fs := afero.NewMemMapFs()
	repoPath := "/workspace/repos/projects/tooling"
	if err := fs.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := WriteStarterRepoConfig(fs, repoPath, "tooling", tokens.RepoKindProject); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	payload, err := afero.ReadFile(fs, filepath.Join(repoPath, "ocean.config.json"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var got starterConfigFile
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Name != "tooling" {
		t.Errorf("name: got %q", got.Name)
	}
	if got.Language != "" {
		t.Errorf("language should default to empty, got %q", got.Language)
	}
	if got.Type != "project" {
		t.Errorf("type: got %q", got.Type)
	}
	zero := starterConfigTask{}
	if got.Tasks != zero {
		t.Errorf("tasks should be empty strings, got %+v", got.Tasks)
	}
}

func TestWriteStarterRepoConfig_TypeFromKind(t *testing.T) {
	cases := []struct {
		kind string
		typ  string
	}{
		{tokens.RepoKindProject, "project"},
		{tokens.RepoKindLibrary, "library"},
		{tokens.RepoKindApp, "app"},
		{tokens.RepoKindService, "service"},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			repoPath := "/workspace/repo"
			if err := fs.MkdirAll(repoPath, 0o755); err != nil {
				t.Fatalf("setup: %v", err)
			}
			if err := WriteStarterRepoConfig(fs, repoPath, "x", tc.kind); err != nil {
				t.Fatalf("write: %v", err)
			}
			payload, _ := afero.ReadFile(fs, filepath.Join(repoPath, "ocean.config.json"))
			if !strings.Contains(string(payload), "\"type\": \""+tc.typ+"\"") {
				t.Errorf("expected type %q in payload, got %s", tc.typ, payload)
			}
		})
	}
}

func TestWriteStarterRepoConfig_RefusesOverwrite(t *testing.T) {
	fs := afero.NewMemMapFs()
	repoPath := "/workspace/repos/projects/tooling"
	if err := fs.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := afero.WriteFile(fs, filepath.Join(repoPath, "ocean.config.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	err := WriteStarterRepoConfig(fs, repoPath, "tooling", tokens.RepoKindProject)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("unexpected error: %v", err)
	}
}
