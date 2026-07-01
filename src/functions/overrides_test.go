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

// #148 Group override of an unknown base task is a validation error
func TestReq148_UnknownBaseTaskIsError(t *testing.T) {
	c := models.RepoConfig{}
	c.Tasks.Build = "go build ./..."
	c.Overrides = []models.OverrideGroup{
		{Group: "desktop", Tasks: models.TaskOverlay{Publish: "npm publish"}},
	}
	_, err := ValidateOverrides(c)
	var ve *oceanerrors.OverridesValidationError
	if !errors.As(err, &ve) || ve.Kind != oceanerrors.KindUnknownBaseTask {
		t.Fatalf("expected unknown_base_task validation error, got %v", err)
	}
	if ve.Task != "publish" {
		t.Fatalf("expected offending task 'publish', got %q", ve.Task)
	}
}

// #149 --group selects a group's task command
func TestReq149_GroupSelectsGroupCommand(t *testing.T) {
	c := desktopConfig()
	groups, _ := ValidateOverrides(c)
	resolved, err := ResolveGroupCommand("build", SelectGroup("desktop"), c, groups, emptyCtx())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Command != "make desktop" {
		t.Fatalf("expected group build command, got %q", resolved.Command)
	}
	if resolved.Source != "group" {
		t.Fatalf("expected source=group, got %q", resolved.Source)
	}
}

// #150 Unlisted task under a group inherits the base command
func TestReq150_UnlistedTaskInheritsBase(t *testing.T) {
	c := desktopConfig()
	groups, _ := ValidateOverrides(c)
	resolved, err := ResolveGroupCommand("check", SelectGroup("desktop"), c, groups, emptyCtx())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Command != "go test ./..." {
		t.Fatalf("expected inherited base check command, got %q", resolved.Command)
	}
	if resolved.Source != "base" {
		t.Fatalf("expected source=base for inherited task, got %q", resolved.Source)
	}
}

// #151 No group selected runs the base command
func TestReq151_NoGroupRunsBase(t *testing.T) {
	c := desktopConfig()
	groups, _ := ValidateOverrides(c)
	resolved, err := ResolveGroupCommand("build", SelectGroup(""), c, groups, emptyCtx())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Command != "go build ./..." {
		t.Fatalf("expected base build command in base mode, got %q", resolved.Command)
	}
	if resolved.Source != "base" {
		t.Fatalf("expected source=base, got %q", resolved.Source)
	}
}

// #152 Unknown --group value is a hard error
func TestReq152_UnknownGroupIsHardError(t *testing.T) {
	c := desktopConfig()
	groups, _ := ValidateOverrides(c)
	_, err := ResolveGroupCommand("build", SelectGroup("web"), c, groups, emptyCtx())
	var ue *oceanerrors.UnknownGroupError
	if !errors.As(err, &ue) {
		t.Fatalf("expected UnknownGroupError, got %v", err)
	}
	if ue.Group != "web" {
		t.Fatalf("expected offending group 'web', got %q", ue.Group)
	}
}

// #153 Overrides apply to non-build lifecycle tasks
func TestReq153_OverrideAppliesToContain(t *testing.T) {
	c := baseConfig()
	c.Overrides = []models.OverrideGroup{
		{Group: "desktop", Tasks: models.TaskOverlay{Contain: "docker build -f Desktop.Dockerfile ."}},
	}
	groups, _ := ValidateOverrides(c)
	resolved, err := ResolveGroupCommand("contain", SelectGroup("desktop"), c, groups, emptyCtx())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Command != "docker build -f Desktop.Dockerfile ." {
		t.Fatalf("expected group contain command, got %q", resolved.Command)
	}
	if resolved.Source != "group" {
		t.Fatalf("expected source=group for contain override, got %q", resolved.Source)
	}
}
