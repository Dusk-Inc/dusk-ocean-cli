package functions

import (
	"errors"
	"testing"

	oceanerrors "github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/errors"
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

// #146 Duplicate group name is a validation error
func TestReq146_DuplicateGroupIsError(t *testing.T) {
	c := baseConfig()
	c.Overrides = []models.OverrideGroup{
		{Group: "desktop", Tasks: models.TaskOverlay{Build: "a"}},
		{Group: "desktop", Tasks: models.TaskOverlay{Build: "b"}},
	}
	_, err := ValidateOverrides(c)
	var ve *oceanerrors.OverridesValidationError
	if !errors.As(err, &ve) || ve.Kind != oceanerrors.KindDuplicateGroup {
		t.Fatalf("expected duplicate_group validation error, got %v", err)
	}
}

// #147 Group entry with no name is a validation error
func TestReq147_EmptyGroupNameIsError(t *testing.T) {
	c := baseConfig()
	c.Overrides = []models.OverrideGroup{{Group: "", Tasks: models.TaskOverlay{Build: "a"}}}
	_, err := ValidateOverrides(c)
	var ve *oceanerrors.OverridesValidationError
	if !errors.As(err, &ve) || ve.Kind != oceanerrors.KindEmptyGroupName {
		t.Fatalf("expected empty_group_name validation error, got %v", err)
	}
}
