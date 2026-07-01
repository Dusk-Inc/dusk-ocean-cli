package functions

import (
	"testing"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/models"
	"github.com/spf13/afero"
)

func newManifestFsWithEntry(t *testing.T, key string) (afero.Fs, string) {
	t.Helper()
	fs := afero.NewMemMapFs()
	root := "/workspace"
	m := Manifest{Repos: map[string]ManifestEntry{
		key: {Kind: "project", Name: "app"},
	}}
	if err := WriteManifest(fs, root, m); err != nil {
		t.Fatalf("setup manifest: %v", err)
	}
	return fs, root
}

func baseConfig() models.RepoConfig {
	c := models.RepoConfig{}
	c.Tasks.Build = "go build ./..."
	c.Tasks.Check = "go test ./..."
	c.Tasks.Contain = "docker build ."
	c.Tasks.Run = "go run ."
	c.Tasks.Stop = "pkill app"
	return c
}

func desktopConfig() models.RepoConfig {
	c := baseConfig()
	overlay := models.TaskOverlay{Build: "make desktop"}
	c.Overrides = []models.OverrideGroup{{Group: "desktop", Tasks: overlay}}
	return c
}

func emptyCtx() VariableContext {
	return VariableContext{
		Env:   map[string]string{},
		Var:   map[string]string{},
		Ocean: map[string]string{},
		Repo:  map[string]string{},
	}
}

// #145 Valid overrides config parses and registers group
func TestReq145_ValidOverridesRegistersGroup(t *testing.T) {
	groups, err := ValidateOverrides(desktopConfig())
	if err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}
	if len(groups) != 1 || groups[0].Group != "desktop" {
		t.Fatalf("expected desktop group registered, got %+v", groups)
	}
	if groups[0].Tasks.Build != "make desktop" {
		t.Fatalf("expected overlay build command retained, got %q", groups[0].Tasks.Build)
	}
}
