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

// #154 Group commands resolve tokens like base tasks
func TestReq154_GroupCommandExpandsTokens(t *testing.T) {
	c := baseConfig()
	c.Overrides = []models.OverrideGroup{
		{Group: "desktop", Tasks: models.TaskOverlay{Build: "build --env {{env:MODE}} --org {{var:org}} --name {{repo:name}}"}},
	}
	groups, _ := ValidateOverrides(c)
	ctx := VariableContext{
		Env:   map[string]string{"MODE": "prod"},
		Var:   map[string]string{"org": "dusk"},
		Ocean: map[string]string{},
		Repo:  map[string]string{"name": "app"},
	}
	resolved, err := ResolveGroupCommand("build", SelectGroup("desktop"), c, groups, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "build --env prod --org dusk --name app"
	if resolved.Command != want {
		t.Fatalf("expected expanded command %q, got %q", want, resolved.Command)
	}
}

// #155 Operation under a group writes the per-(repo,group) hash slot
func TestReq155_GroupOpWritesGroupSlot(t *testing.T) {
	fs, root := newManifestFsWithEntry(t, "project:app")
	slot, err := WriteGroupCacheSlot(fs, root, "project:app", SelectGroup("desktop"), "build", "hash-desktop", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if slot == nil || slot.BuildHash != "hash-desktop" || slot.Group != "desktop" {
		t.Fatalf("expected desktop slot with build hash, got %+v", slot)
	}
	m, _ := ReadManifest(fs, root)
	got := m.Repos["project:app"].Groups["desktop"]
	if got.BuildHash != "hash-desktop" {
		t.Fatalf("expected persisted desktop build hash, got %q", got.BuildHash)
	}
}

// #156 Base mode uses a slot distinct from any group's
func TestReq156_BaseSlotDistinctFromGroup(t *testing.T) {
	fs, root := newManifestFsWithEntry(t, "project:app")
	if _, err := WriteGroupCacheSlot(fs, root, "project:app", SelectGroup(""), "build", "hash-base", true); err != nil {
		t.Fatalf("base write: %v", err)
	}
	if _, err := WriteGroupCacheSlot(fs, root, "project:app", SelectGroup("desktop"), "build", "hash-desktop", true); err != nil {
		t.Fatalf("desktop write: %v", err)
	}
	m, _ := ReadManifest(fs, root)
	entry := m.Repos["project:app"]
	if entry.BuildHash != "hash-base" {
		t.Fatalf("expected base slot hash 'hash-base', got %q", entry.BuildHash)
	}
	if entry.Groups["desktop"].BuildHash != "hash-desktop" {
		t.Fatalf("expected desktop slot hash 'hash-desktop', got %q", entry.Groups["desktop"].BuildHash)
	}
	if entry.BuildHash == entry.Groups["desktop"].BuildHash {
		t.Fatalf("base and group slot must be distinct")
	}
}

// #157 Changing one group's override invalidates only that group's cache
func TestReq157_ChangeInvalidatesOnlyThatGroup(t *testing.T) {
	fs, root := newManifestFsWithEntry(t, "project:app")
	WriteGroupCacheSlot(fs, root, "project:app", SelectGroup(""), "build", "hash-base", true)
	WriteGroupCacheSlot(fs, root, "project:app", SelectGroup("desktop"), "build", "hash-desktop-v1", true)
	WriteGroupCacheSlot(fs, root, "project:app", SelectGroup("web"), "build", "hash-web", true)

	// desktop override changes -> rewrite desktop slot only
	WriteGroupCacheSlot(fs, root, "project:app", SelectGroup("desktop"), "build", "hash-desktop-v2", true)

	m, _ := ReadManifest(fs, root)
	entry := m.Repos["project:app"]
	if entry.Groups["desktop"].BuildHash != "hash-desktop-v2" {
		t.Fatalf("expected desktop slot rewritten, got %q", entry.Groups["desktop"].BuildHash)
	}
	if entry.BuildHash != "hash-base" {
		t.Fatalf("base slot must be untouched, got %q", entry.BuildHash)
	}
	if entry.Groups["web"].BuildHash != "hash-web" {
		t.Fatalf("sibling web slot must be untouched, got %q", entry.Groups["web"].BuildHash)
	}
}

// #158 A matching group-slot hash skips the operation as fresh
func TestReq158_MatchingGroupHashIsFresh(t *testing.T) {
	fs, root := newManifestFsWithEntry(t, "project:app")
	WriteGroupCacheSlot(fs, root, "project:app", SelectGroup("desktop"), "build", "hash-desktop", true)
	fresh, slot, err := ReadGroupCacheSlot(fs, root, "project:app", SelectGroup("desktop"), "build", "hash-desktop")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fresh {
		t.Fatalf("expected fresh=true for matching desktop hash")
	}
	if slot == nil || slot.BuildHash != "hash-desktop" {
		t.Fatalf("expected returned desktop slot, got %+v", slot)
	}
}

// #159 A missing or mismatched group-slot hash rebuilds
func TestReq159_MissingOrMismatchedGroupHashRebuilds(t *testing.T) {
	fs, root := newManifestFsWithEntry(t, "project:app")
	// missing: no slot written yet
	fresh, _, err := ReadGroupCacheSlot(fs, root, "project:app", SelectGroup("desktop"), "build", "hash-desktop")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fresh {
		t.Fatalf("expected fresh=false for missing desktop slot")
	}
	// mismatched: slot exists with a different hash
	WriteGroupCacheSlot(fs, root, "project:app", SelectGroup("desktop"), "build", "hash-old", true)
	fresh, _, err = ReadGroupCacheSlot(fs, root, "project:app", SelectGroup("desktop"), "build", "hash-new")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fresh {
		t.Fatalf("expected fresh=false for mismatched desktop slot")
	}
	// contrast: the same slot, queried with its stored hash, must rebuild no longer
	fresh, _, err = ReadGroupCacheSlot(fs, root, "project:app", SelectGroup("desktop"), "build", "hash-old")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fresh {
		t.Fatalf("a written slot must be fresh for its own hash; the read must compare, not always-rebuild")
	}
}
