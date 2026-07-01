//go:build integration

package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These tests exercise Feature #65 "Deployment-mode lifecycle overrides" end-to-end
// against real dependencies: the real dusk-ocean binary, real ocean.config.json parsing
// on the OS filesystem, and real .ocean/manifest.json reads/writes. They deliberately
// replace dev's afero.NewMemMapFs() unit mocks with the live filesystem and the real CLI.
//
// The binary path is injected via OCEAN_BIN (the Ocean run task builds it first); tests
// never hardcode a build path and never call internal override functions directly.

func bin(t *testing.T) string {
	t.Helper()
	b := os.Getenv("OCEAN_BIN")
	if b == "" {
		t.Skip("OCEAN_BIN not set; integration harness must build and inject the real binary")
	}
	return b
}

// newRepoWorkspace stands up a real ephemeral single-project workspace on the OS filesystem
// with the given repo ocean.config.json body, returning the workspace root.
func newRepoWorkspace(t *testing.T, configJSON string) string {
	t.Helper()
	root := t.TempDir()
	repoRel := filepath.Join("repos", "projects", "app")
	repoDir := filepath.Join(root, repoRel)
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "ocean.config.json"), []byte(configJSON), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	ws := `{"workspace":"itest","variables":{"org":"dusk"},"tasks":{},"apps":[],"libraries":[],"projects":[{"name":"app","path":"repos/projects/app"}]}`
	if err := os.WriteFile(filepath.Join(root, "ocean.workspace.json"), []byte(ws), 0o644); err != nil {
		t.Fatalf("write workspace: %v", err)
	}
	return root
}

type runResult struct {
	stdout   string
	stderr   string
	combined string
	err      error
	exitCode int
}

// invoke runs the real binary from the workspace root with the given args.
func invoke(t *testing.T, root string, env []string, args ...string) runResult {
	t.Helper()
	cmd := exec.Command(bin(t), args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	res := runResult{combined: string(out), err: err}
	if ee, ok := err.(*exec.ExitError); ok {
		res.exitCode = ee.ExitCode()
	}
	return res
}

func readManifest(t *testing.T, root string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".ocean", "manifest.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}
		}
		t.Fatalf("read manifest: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	return m
}

const baseTasks = `"tasks":{"build":"echo BASE_BUILD","check":"echo BASE_CHECK","contain":"echo BASE_CONTAIN","run":"echo BASE_RUN","stop":"echo BASE_STOP"}`

func cfg(overridesJSON string) string {
	return `{"name":"app","language":"go","type":"project",` + baseTasks + `,"overrides":` + overridesJSON + `}`
}

// --- #145 valid overrides config parses/registers group ---
// Then: a base lifecycle op that consults the registered group must succeed and the group
// must be usable via --group (its build command resolvable end-to-end).
func TestReq145_ValidOverridesParsesAndRegistersGroup(t *testing.T) {
	root := newRepoWorkspace(t, cfg(`[{"group":"desktop","tasks":{"build":"echo DESKTOP_BUILD"}}]`))
	r := invoke(t, root, nil, "build", "project", "--name", "app", "--group", "desktop")
	if r.err != nil {
		t.Fatalf("valid overrides config must parse and the desktop group be selectable end-to-end; got error %v\noutput:\n%s", r.err, r.combined)
	}
	if !strings.Contains(r.combined, "DESKTOP_BUILD") {
		t.Fatalf("registered desktop group's build command must run; got:\n%s", r.combined)
	}
}

// --- #146 duplicate group name = validation error ---
func TestReq146_DuplicateGroupIsValidationError(t *testing.T) {
	root := newRepoWorkspace(t, cfg(`[{"group":"desktop","tasks":{"build":"echo A"}},{"group":"desktop","tasks":{"build":"echo B"}}]`))
	r := invoke(t, root, nil, "build", "project", "--name", "app")
	if r.err == nil {
		t.Fatalf("a config with a duplicate group name must be rejected as a validation error; command succeeded, output:\n%s", r.combined)
	}
	if !strings.Contains(strings.ToLower(r.combined), "duplicate") {
		t.Fatalf("error must name the duplicate-group validation failure; got:\n%s", r.combined)
	}
}

// --- #147 empty group name = validation error ---
func TestReq147_EmptyGroupNameIsValidationError(t *testing.T) {
	root := newRepoWorkspace(t, cfg(`[{"group":"","tasks":{"build":"echo A"}}]`))
	r := invoke(t, root, nil, "build", "project", "--name", "app")
	if r.err == nil {
		t.Fatalf("an overrides entry with an empty group name must be rejected; command succeeded, output:\n%s", r.combined)
	}
	if !strings.Contains(strings.ToLower(r.combined), "name") {
		t.Fatalf("error must name the empty-group-name validation failure; got:\n%s", r.combined)
	}
}

// --- #148 overlay of unknown base task = validation error ---
func TestReq148_UnknownBaseTaskIsValidationError(t *testing.T) {
	// desktop overlays "publish" which the base tasks map does not declare.
	root := newRepoWorkspace(t, cfg(`[{"group":"desktop","tasks":{"publish":"echo PUB"}}]`))
	r := invoke(t, root, nil, "build", "project", "--name", "app")
	if r.err == nil {
		t.Fatalf("a group overlaying an unknown base task must be rejected; command succeeded, output:\n%s", r.combined)
	}
	if !strings.Contains(strings.ToLower(r.combined), "publish") {
		t.Fatalf("error must name the offending task; got:\n%s", r.combined)
	}
}

// --- #149 --group selects group's command ---
func TestReq149_GroupSelectsGroupCommand(t *testing.T) {
	root := newRepoWorkspace(t, cfg(`[{"group":"desktop","tasks":{"build":"echo DESKTOP_BUILD"}}]`))
	r := invoke(t, root, nil, "build", "project", "--name", "app", "--group", "desktop")
	if r.err != nil {
		t.Fatalf("--group desktop must be accepted; got error %v\noutput:\n%s", r.err, r.combined)
	}
	if !strings.Contains(r.combined, "DESKTOP_BUILD") || strings.Contains(r.combined, "BASE_BUILD") {
		t.Fatalf("--group desktop must run the group's build command, not the base; got:\n%s", r.combined)
	}
}

// --- #150 unlisted task inherits base ---
func TestReq150_UnlistedTaskInheritsBase(t *testing.T) {
	// desktop overrides build but not check; check under --group desktop must run base check.
	root := newRepoWorkspace(t, cfg(`[{"group":"desktop","tasks":{"build":"echo DESKTOP_BUILD"}}]`))
	r := invoke(t, root, nil, "check", "project", "--name", "app", "--group", "desktop")
	if r.err != nil {
		t.Fatalf("check --group desktop must be accepted; got error %v\noutput:\n%s", r.err, r.combined)
	}
	if !strings.Contains(r.combined, "BASE_CHECK") {
		t.Fatalf("a task unlisted in the group must inherit the base command; got:\n%s", r.combined)
	}
}

// --- #151 no group runs base ---
func TestReq151_NoGroupRunsBase(t *testing.T) {
	root := newRepoWorkspace(t, cfg(`[{"group":"desktop","tasks":{"build":"echo DESKTOP_BUILD"}}]`))
	r := invoke(t, root, nil, "build", "project", "--name", "app")
	if r.err != nil {
		t.Fatalf("base build must succeed; got error %v\noutput:\n%s", r.err, r.combined)
	}
	if !strings.Contains(r.combined, "BASE_BUILD") || strings.Contains(r.combined, "DESKTOP_BUILD") {
		t.Fatalf("with no --group the base command must run; got:\n%s", r.combined)
	}
}

// --- #152 unknown --group = hard error ---
func TestReq152_UnknownGroupIsHardError(t *testing.T) {
	root := newRepoWorkspace(t, cfg(`[{"group":"desktop","tasks":{"build":"echo DESKTOP_BUILD"}}]`))
	r := invoke(t, root, nil, "build", "project", "--name", "app", "--group", "web")
	if r.err == nil {
		t.Fatalf("an unknown --group value must be a hard error; command succeeded, output:\n%s", r.combined)
	}
	if !strings.Contains(r.combined, "web") {
		t.Fatalf("hard error must name the unknown group; got:\n%s", r.combined)
	}
}

// --- #153 overrides apply to non-build lifecycle tasks ---
func TestReq153_OverrideAppliesToContain(t *testing.T) {
	root := newRepoWorkspace(t, cfg(`[{"group":"desktop","tasks":{"contain":"echo DESKTOP_CONTAIN"}}]`))
	r := invoke(t, root, nil, "contain", "project", "--name", "app", "--group", "desktop")
	if r.err != nil {
		t.Fatalf("contain --group desktop must be accepted; got error %v\noutput:\n%s", r.err, r.combined)
	}
	if !strings.Contains(r.combined, "DESKTOP_CONTAIN") {
		t.Fatalf("a group override must apply to a non-build lifecycle task; got:\n%s", r.combined)
	}
}

// --- #154 group commands resolve tokens like base ---
func TestReq154_GroupCommandResolvesTokens(t *testing.T) {
	root := newRepoWorkspace(t, cfg(`[{"group":"desktop","tasks":{"build":"echo MODE-{{env:MODE}}-{{var:org}}"}}]`))
	r := invoke(t, root, []string{"MODE=prod"}, "build", "project", "--name", "app", "--group", "desktop")
	if r.err != nil {
		t.Fatalf("group build with tokens must be accepted; got error %v\noutput:\n%s", r.err, r.combined)
	}
	if !strings.Contains(r.combined, "MODE-prod-dusk") {
		t.Fatalf("group command must resolve {{env:*}}/{{var:*}} tokens like a base task; got:\n%s", r.combined)
	}
}

// --- #155 group op writes per-(repo,group) hash slot ---
func TestReq155_GroupOpWritesGroupSlot(t *testing.T) {
	root := newRepoWorkspace(t, cfg(`[{"group":"desktop","tasks":{"build":"echo DESKTOP_BUILD"}}]`))
	if r := invoke(t, root, nil, "build", "project", "--name", "app", "--group", "desktop"); r.err != nil {
		t.Fatalf("group build must be accepted; got error %v\noutput:\n%s", r.err, r.combined)
	}
	m := readManifest(t, root)
	repos, _ := m["repos"].(map[string]any)
	found := false
	for _, v := range repos {
		e, _ := v.(map[string]any)
		groups, ok := e["groups"].(map[string]any)
		if ok {
			if _, has := groups["desktop"]; has {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("a build under --group desktop must write a per-(repo,group) slot in the real manifest; manifest:\n%v", m)
	}
}

// --- #156 base slot distinct from any group's ---
func TestReq156_BaseSlotDistinctFromGroup(t *testing.T) {
	root := newRepoWorkspace(t, cfg(`[{"group":"desktop","tasks":{"build":"echo DESKTOP_BUILD"}}]`))
	if r := invoke(t, root, nil, "build", "project", "--name", "app"); r.err != nil {
		t.Fatalf("base build must be accepted; %v\n%s", r.err, r.combined)
	}
	if r := invoke(t, root, nil, "build", "project", "--name", "app", "--group", "desktop"); r.err != nil {
		t.Fatalf("group build must be accepted; %v\n%s", r.err, r.combined)
	}
	m := readManifest(t, root)
	repos, _ := m["repos"].(map[string]any)
	for _, v := range repos {
		e, _ := v.(map[string]any)
		baseHash, _ := e["build_hash"].(string)
		groups, _ := e["groups"].(map[string]any)
		g, _ := groups["desktop"].(map[string]any)
		groupHash, _ := g["build_hash"].(string)
		if baseHash == "" || groupHash == "" {
			t.Fatalf("both base and desktop build hashes must be populated; base=%q group=%q", baseHash, groupHash)
		}
		if baseHash == groupHash {
			t.Fatalf("base slot must be distinct from the group slot; both were %q", baseHash)
		}
		return
	}
	t.Fatalf("expected a manifest repo entry; got:\n%v", m)
}

// --- #157 changing one group's override invalidates only that group's cache ---
func TestReq157_ChangeInvalidatesOnlyThatGroup(t *testing.T) {
	root := newRepoWorkspace(t, cfg(`[{"group":"desktop","tasks":{"build":"echo DESKTOP_V1"}},{"group":"web","tasks":{"build":"echo WEB_BUILD"}}]`))
	invoke(t, root, nil, "build", "project", "--name", "app")
	invoke(t, root, nil, "build", "project", "--name", "app", "--group", "desktop")
	if r := invoke(t, root, nil, "build", "project", "--name", "app", "--group", "web"); r.err != nil {
		t.Fatalf("web group build must be accepted; %v\n%s", r.err, r.combined)
	}
	before := readManifest(t, root)
	webHashBefore, baseHashBefore := groupHash(before, "web"), baseBuildHash(before)

	// change only desktop's build command
	root2cfg := cfg(`[{"group":"desktop","tasks":{"build":"echo DESKTOP_V2"}},{"group":"web","tasks":{"build":"echo WEB_BUILD"}}]`)
	if err := os.WriteFile(filepath.Join(root, "repos", "projects", "app", "ocean.config.json"), []byte(root2cfg), 0o644); err != nil {
		t.Fatalf("rewrite config: %v", err)
	}
	if r := invoke(t, root, nil, "build", "project", "--name", "app", "--group", "desktop"); r.err != nil {
		t.Fatalf("desktop rebuild must be accepted; %v\n%s", r.err, r.combined)
	}
	after := readManifest(t, root)
	if groupHash(after, "web") != webHashBefore {
		t.Fatalf("web slot must be untouched when only desktop changed")
	}
	if baseBuildHash(after) != baseHashBefore {
		t.Fatalf("base slot must be untouched when only desktop changed")
	}
	if groupHash(after, "desktop") == "" {
		t.Fatalf("desktop slot must be present/updated after its override changed")
	}
}

// --- #158 matching group-slot hash skips as fresh ---
func TestReq158_MatchingGroupHashSkipsFresh(t *testing.T) {
	root := newRepoWorkspace(t, cfg(`[{"group":"desktop","tasks":{"build":"echo DESKTOP_BUILD"}}]`))
	if r := invoke(t, root, nil, "build", "project", "--name", "app", "--group", "desktop"); r.err != nil {
		t.Fatalf("first desktop build must be accepted; %v\n%s", r.err, r.combined)
	}
	r := invoke(t, root, nil, "build", "project", "--name", "app", "--group", "desktop")
	if r.err != nil {
		t.Fatalf("second desktop build must be accepted; %v\n%s", r.err, r.combined)
	}
	low := strings.ToLower(r.combined)
	if !(strings.Contains(low, "fresh") || strings.Contains(low, "skip") || strings.Contains(low, "up to date") || strings.Contains(low, "cached")) {
		t.Fatalf("a matching desktop group-slot hash must skip the op as fresh; second run did not report a cache hit:\n%s", r.combined)
	}
}

// --- #159 missing/mismatched group-slot hash rebuilds ---
func TestReq159_MismatchedGroupHashRebuilds(t *testing.T) {
	root := newRepoWorkspace(t, cfg(`[{"group":"desktop","tasks":{"build":"echo DESKTOP_V1"}}]`))
	if r := invoke(t, root, nil, "build", "project", "--name", "app", "--group", "desktop"); r.err != nil {
		t.Fatalf("first desktop build must be accepted; %v\n%s", r.err, r.combined)
	}
	// change desktop's build command so the resolved hash no longer matches
	if err := os.WriteFile(filepath.Join(root, "repos", "projects", "app", "ocean.config.json"),
		[]byte(cfg(`[{"group":"desktop","tasks":{"build":"echo DESKTOP_V2"}}]`)), 0o644); err != nil {
		t.Fatalf("rewrite config: %v", err)
	}
	r := invoke(t, root, nil, "build", "project", "--name", "app", "--group", "desktop")
	if r.err != nil {
		t.Fatalf("desktop rebuild must be accepted; %v\n%s", r.err, r.combined)
	}
	if !strings.Contains(r.combined, "DESKTOP_V2") {
		t.Fatalf("a mismatched desktop group-slot hash must trigger a rebuild running the new command; got:\n%s", r.combined)
	}
}

// --- #160 run honors --group but writes no cache slot ---
func TestReq160_RunHonorsGroupNoSlot(t *testing.T) {
	root := newRepoWorkspace(t, cfg(`[{"group":"desktop","tasks":{"run":"echo RUN_DESKTOP"}}]`))
	r := invoke(t, root, nil, "run", "project", "--name", "app", "--group", "desktop")
	if r.err != nil {
		t.Fatalf("run --group desktop must be accepted; got error %v\noutput:\n%s", r.err, r.combined)
	}
	if !strings.Contains(r.combined, "RUN_DESKTOP") {
		t.Fatalf("run must honor the group's run command; got:\n%s", r.combined)
	}
	if groupHash(readManifest(t, root), "desktop") != "" {
		t.Fatalf("run must write no cache slot for the group")
	}
}

// --- #161 stop honors --group but writes no cache slot ---
func TestReq161_StopHonorsGroupNoSlot(t *testing.T) {
	root := newRepoWorkspace(t, cfg(`[{"group":"desktop","tasks":{"stop":"echo STOP_DESKTOP"}}]`))
	r := invoke(t, root, nil, "stop", "project", "--name", "app", "--group", "desktop")
	if r.err != nil {
		t.Fatalf("stop --group desktop must be accepted; got error %v\noutput:\n%s", r.err, r.combined)
	}
	if !strings.Contains(r.combined, "STOP_DESKTOP") {
		t.Fatalf("stop must honor the group's stop command; got:\n%s", r.combined)
	}
	if groupHash(readManifest(t, root), "desktop") != "" {
		t.Fatalf("stop must write no cache slot for the group")
	}
}

func baseBuildHash(m map[string]any) string {
	repos, _ := m["repos"].(map[string]any)
	for _, v := range repos {
		e, _ := v.(map[string]any)
		h, _ := e["build_hash"].(string)
		return h
	}
	return ""
}

func groupHash(m map[string]any, group string) string {
	repos, _ := m["repos"].(map[string]any)
	for _, v := range repos {
		e, _ := v.(map[string]any)
		groups, _ := e["groups"].(map[string]any)
		g, _ := groups[group].(map[string]any)
		h, _ := g["build_hash"].(string)
		if h != "" {
			return h
		}
	}
	return ""
}
