package functions

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
	"github.com/spf13/afero"
)

func TestSubstitute_AllNamespacesResolve(t *testing.T) {
	ctx := VariableContext{
		Env:   map[string]string{"TOKEN": "secret"},
		Var:   map[string]string{"org": "dusk-inc"},
		Ocean: map[string]string{"port": "3000"},
		Repo:  map[string]string{"name": "svc-a"},
	}
	template := "{{env:TOKEN}} {{var:org}} {{ocean:port}} {{repo:name}}"
	got, err := Substitute(template, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "secret dusk-inc 3000 svc-a"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSubstitute_UnknownNamespaceErrors(t *testing.T) {
	ctx := VariableContext{Repo: map[string]string{"name": "x"}}
	_, err := Substitute("{{wat:foo}}", ctx)
	if err == nil {
		t.Fatal("expected error for unknown namespace")
	}
	if !strings.Contains(err.Error(), "unknown variable namespace") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSubstitute_MissingKeyErrors(t *testing.T) {
	ctx := VariableContext{Repo: map[string]string{}}
	_, err := Substitute("hello {{repo:name}}", ctx)
	if err == nil {
		t.Fatal("expected error for missing key")
	}
	if !strings.Contains(err.Error(), "missing variable repo:name") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSubstitute_NoTokensIsIdentity(t *testing.T) {
	got, err := Substitute("plain text with no tokens", VariableContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "plain text with no tokens" {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestSubstitute_RepeatedTokens(t *testing.T) {
	ctx := VariableContext{Repo: map[string]string{"name": "svc"}}
	got, err := Substitute("{{repo:name}}-{{repo:name}}", ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "svc-svc" {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestLoadEnvFile_MissingFileLogsAndReturnsEmpty(t *testing.T) {
	fs := afero.NewMemMapFs()
	var out bytes.Buffer
	values, err := LoadEnvFile(fs, "/workspace", &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(values) != 0 {
		t.Fatalf("expected empty map, got %v", values)
	}
	if !strings.Contains(out.String(), ".env not found") {
		t.Fatalf("expected absence log line, got %q", out.String())
	}
}

func TestLoadEnvFile_ParsesKeyValueWithCommentsAndBlanks(t *testing.T) {
	fs := afero.NewMemMapFs()
	content := "# a comment\n\nFOO=bar\nGITHUB_TOKEN=ghp_abc\n  # indented comment ignored as well\nBAZ=qux=quux\n"
	if err := afero.WriteFile(fs, "/workspace/.env", []byte(content), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	values, err := LoadEnvFile(fs, "/workspace", &bytes.Buffer{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]string{
		"FOO":          "bar",
		"GITHUB_TOKEN": "ghp_abc",
		"BAZ":          "qux=quux",
	}
	if len(values) != len(want) {
		t.Fatalf("got %d entries, want %d (%v)", len(values), len(want), values)
	}
	for k, v := range want {
		if values[k] != v {
			t.Fatalf("key %s: got %q want %q", k, values[k], v)
		}
	}
}

func TestLoadEnvFile_RejectsLineWithoutEquals(t *testing.T) {
	fs := afero.NewMemMapFs()
	if err := afero.WriteFile(fs, "/workspace/.env", []byte("INVALID_LINE\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	_, err := LoadEnvFile(fs, "/workspace", &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error for malformed .env line")
	}
}

func TestBuildRepoVariables_ProjectReservedFields(t *testing.T) {
	cfg := WorkspaceConfig{
		Projects: []WorkspaceProject{
			{
				Name:   "tooling",
				Remote: "git@github.com:dusk-inc/tooling.git",
				Scopes: []string{"shared", "internal"},
			},
		},
	}
	got, err := BuildRepoVariables(cfg, tokens.RepoKindProject, "", "tooling")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["name"] != "tooling" {
		t.Errorf("name: got %q", got["name"])
	}
	if got["kind"] != tokens.RepoKindProject {
		t.Errorf("kind: got %q", got["kind"])
	}
	if got["path"] != "repos/projects/tooling" {
		t.Errorf("path: got %q", got["path"])
	}
	if got["remote"] != "git@github.com:dusk-inc/tooling.git" {
		t.Errorf("remote: got %q", got["remote"])
	}
	if got["scopes"] != "shared,internal" {
		t.Errorf("scopes: got %q", got["scopes"])
	}
}

func TestBuildRepoVariables_GlobalLibrary(t *testing.T) {
	cfg := WorkspaceConfig{
		Libraries: []WorkspaceLibrary{
			{Name: "lib-a", Remote: "git@github.com:dusk-inc/lib-a.git"},
		},
	}
	got, err := BuildRepoVariables(cfg, tokens.RepoKindLibrary, "", "lib-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["path"] != "repos/libs/lib-a" {
		t.Errorf("path: got %q", got["path"])
	}
	if _, hasApp := got["app"]; hasApp {
		t.Errorf("global library should not expose app variable")
	}
}

func TestBuildRepoVariables_AppScopedLibrary(t *testing.T) {
	cfg := WorkspaceConfig{
		Apps: []WorkspaceApp{
			{
				Name: "app-a",
				Libraries: []WorkspaceLibrary{
					{Name: "lib-a", Remote: "git@github.com:dusk-inc/lib-a.git"},
				},
			},
		},
	}
	got, err := BuildRepoVariables(cfg, tokens.RepoKindLibrary, "app-a", "lib-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["path"] != "repos/apps/app-a/libs/lib-a" {
		t.Errorf("path: got %q", got["path"])
	}
	if got["app"] != "app-a" {
		t.Errorf("app: got %q", got["app"])
	}
}

func TestBuildRepoVariables_ServiceReservedFields(t *testing.T) {
	cfg := WorkspaceConfig{
		Apps: []WorkspaceApp{
			{
				Name: "app-a",
				Services: []WorkspaceService{
					{
						Name:          "svc-a",
						Port:          "3001",
						Image:         WorkspaceImage{Name: "app-a__svc-a", Tag: "dev"},
						Dockerfile:    "ts.Dockerfile",
						ContainerFile: "ts.Dockerfile",
						ImagePath:     "registry.example.com/app-a/svc-a",
						Remote:        "git@github.com:dusk-inc/svc-a.git",
					},
				},
			},
		},
	}
	got, err := BuildRepoVariables(cfg, tokens.RepoKindService, "app-a", "svc-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	checks := map[string]string{
		"name":           "svc-a",
		"port":           "3001",
		"image_name":     "app-a__svc-a",
		"image_tag":      "dev",
		"dockerfile":     "ts.Dockerfile",
		"container_file": "ts.Dockerfile",
		"image_path":     "registry.example.com/app-a/svc-a",
		"app":            "app-a",
		"path":           "repos/apps/app-a/services/svc-a",
	}
	for k, want := range checks {
		if got[k] != want {
			t.Errorf("%s: got %q want %q", k, got[k], want)
		}
	}
}

func TestBuildRepoVariables_AppKind(t *testing.T) {
	cfg := WorkspaceConfig{
		Apps: []WorkspaceApp{
			{Name: "app-a", Remote: "git@github.com:dusk-inc/app-a.git"},
		},
	}
	got, err := BuildRepoVariables(cfg, tokens.RepoKindApp, "", "app-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["path"] != "repos/apps/app-a" {
		t.Errorf("path: got %q", got["path"])
	}
}

func TestBuildRepoVariables_UserVariablesMergeIn(t *testing.T) {
	cfg := WorkspaceConfig{
		Projects: []WorkspaceProject{
			{
				Name:      "tooling",
				Variables: map[string]string{"branch": "main", "deploy_env": "staging"},
			},
		},
	}
	got, err := BuildRepoVariables(cfg, tokens.RepoKindProject, "", "tooling")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["branch"] != "main" || got["deploy_env"] != "staging" {
		t.Errorf("user variables not merged: got %v", got)
	}
}

func TestBuildRepoVariables_CollisionWithReservedNameRejected(t *testing.T) {
	cfg := WorkspaceConfig{
		Projects: []WorkspaceProject{
			{
				Name:      "tooling",
				Variables: map[string]string{"name": "shadowed"},
			},
		},
	}
	_, err := BuildRepoVariables(cfg, tokens.RepoKindProject, "", "tooling")
	if err == nil {
		t.Fatal("expected collision error")
	}
	if !strings.Contains(err.Error(), "collides with reserved repo field") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildRepoVariables_UnknownKindErrors(t *testing.T) {
	_, err := BuildRepoVariables(WorkspaceConfig{}, "weird-kind", "", "x")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildRepoVariables_RepoNotFound(t *testing.T) {
	cfg := WorkspaceConfig{}
	_, err := BuildRepoVariables(cfg, tokens.RepoKindProject, "", "missing")
	if err == nil {
		t.Fatal("expected error")
	}
}
