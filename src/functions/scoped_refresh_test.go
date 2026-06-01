package functions

import (
	"strings"
	"testing"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
)

// makeScopedConfig builds a workspace with a small, deliberate dependency graph:
//
//	global lib  core            (no deps)
//	global lib  util            -> core
//	project     app-cli         -> util            (so app-cli -> util -> core)
//	project     standalone      (no deps, unrelated)
//	app         web
//	  lib       web-lib         -> core
//	  service   api             -> web-lib
//
// plus non-code infra/docs repos, which are never part of any dependency graph.
func makeScopedConfig() WorkspaceConfig {
	return WorkspaceConfig{
		Workspace: "test",
		Tasks:     map[string]string{tokens.WorkspaceTaskClone: "git clone {{repo:remote}} {{repo:path}}"},
		Apps: []WorkspaceApp{
			{
				Name: "web",
				Libraries: []WorkspaceLibrary{
					{Name: "web-lib", Deps: []WorkspaceDep{{Lib: "core", From: "global"}}},
				},
				Services: []WorkspaceService{
					{Name: "api", Deps: []WorkspaceDep{{Lib: "web-lib", From: "web"}}},
				},
			},
		},
		Libraries: []WorkspaceLibrary{
			{Name: "core"},
			{Name: "util", Deps: []WorkspaceDep{{Lib: "core", From: "global"}}},
		},
		Projects: []WorkspaceProject{
			{Name: "app-cli", Deps: []WorkspaceDep{{Lib: "util", From: "global"}}},
			{Name: "standalone"},
		},
		Infrastructure: []WorkspaceInfra{{Name: "infra-repo", Remote: "https://example.com/infra"}},
		Docs:           []WorkspaceDocs{{Name: "docs-repo", Remote: "https://example.com/docs"}},
	}
}

func keysOf(nodes []Node) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = nodeKey(n)
	}
	return out
}

func indexOfKey(keys []string, key string) int {
	for i, k := range keys {
		if k == key {
			return i
		}
	}
	return -1
}

func mustOrder(t *testing.T, config WorkspaceConfig, repo string, noDeps bool) []string {
	t.Helper()
	nodes, err := ScopedRefreshOrder(config, repo, noDeps)
	if err != nil {
		t.Fatalf("ScopedRefreshOrder(%q, noDeps=%v) error: %v", repo, noDeps, err)
	}
	return keysOf(nodes)
}

func TestScopedRefreshOrder(t *testing.T) {
	t.Run("domain__ScopedRefreshOrder__includesTransitiveDepsInDependencyOrder", func(t *testing.T) {
		keys := mustOrder(t, makeScopedConfig(), "app-cli", false)

		core, util, app := GlobalLibKey("core"), GlobalLibKey("util"), ProjectKey("app-cli")
		for _, want := range []string{core, util, app} {
			if indexOfKey(keys, want) == -1 {
				t.Fatalf("expected %s in scope, got %v", want, keys)
			}
		}
		if !(indexOfKey(keys, core) < indexOfKey(keys, util) && indexOfKey(keys, util) < indexOfKey(keys, app)) {
			t.Fatalf("dependency order violated (want core < util < app-cli): %v", keys)
		}
	})

	t.Run("domain__ScopedRefreshOrder__excludesUnrelatedAndNonCodeRepos", func(t *testing.T) {
		keys := mustOrder(t, makeScopedConfig(), "app-cli", false)

		if indexOfKey(keys, ProjectKey("standalone")) != -1 {
			t.Fatalf("unrelated repo 'standalone' must not be in scope: %v", keys)
		}
		for _, k := range keys {
			if strings.Contains(k, "infra-repo") || strings.Contains(k, "docs-repo") {
				t.Fatalf("non-code repo leaked into scope: %v", keys)
			}
		}
	})

	t.Run("domain__ScopedRefreshOrder__noDepsReturnsOnlyTheNamedRepo", func(t *testing.T) {
		keys := mustOrder(t, makeScopedConfig(), "app-cli", true)

		if len(keys) != 1 || keys[0] != ProjectKey("app-cli") {
			t.Fatalf("--no-deps should return only the named repo, got %v", keys)
		}
	})

	t.Run("domain__ScopedRefreshOrder__appScopeIncludesAllAppNodesAndTransitiveDeps", func(t *testing.T) {
		keys := mustOrder(t, makeScopedConfig(), "web", false)

		core := GlobalLibKey("core")
		webLib := AppLibKey("web", "web-lib")
		api := ServiceKey("web", "api")
		for _, want := range []string{core, webLib, api} {
			if indexOfKey(keys, want) == -1 {
				t.Fatalf("expected %s in scope for app 'web', got %v", want, keys)
			}
		}
		if !(indexOfKey(keys, core) < indexOfKey(keys, webLib) && indexOfKey(keys, webLib) < indexOfKey(keys, api)) {
			t.Fatalf("dependency order violated (want core < web-lib < api): %v", keys)
		}
	})

	t.Run("boundary__ScopedRefreshOrder__repoWithNoDependencies", func(t *testing.T) {
		keys := mustOrder(t, makeScopedConfig(), "core", false)

		if len(keys) != 1 || keys[0] != GlobalLibKey("core") {
			t.Fatalf("a repo with no deps should return only itself, got %v", keys)
		}
	})

	t.Run("boundary__ScopedRefreshOrder__presentAppWithNoComponentsIsEmptyNotError", func(t *testing.T) {
		// An app that exists in config but declares no services/libraries/tests is a
		// valid, empty scope — not the "absent from config" failure mode.
		config := makeScopedConfig()
		config.Apps = append(config.Apps, WorkspaceApp{Name: "empty-app"})

		nodes, err := ScopedRefreshOrder(config, "empty-app", false)
		if err != nil {
			t.Fatalf("a present-but-empty app must not error, got: %v", err)
		}
		if len(nodes) != 0 {
			t.Fatalf("a present-but-empty app should resolve to an empty scope, got %v", keysOf(nodes))
		}
	})
}

func TestScopedRefreshFailureModes(t *testing.T) {
	t.Run("error__ScopedRefreshOrder__unknownRepoErrorNamesTheRepo", func(t *testing.T) {
		_, err := ScopedRefreshOrder(makeScopedConfig(), "ghost", false)
		if err == nil {
			t.Fatalf("expected an error for an unknown repo")
		}
		if !strings.Contains(err.Error(), "ghost") {
			t.Fatalf("error must name the unknown repo, got: %v", err)
		}
	})

	t.Run("error__ScopedRefreshOrder__cycleAmongTargetedReposFails", func(t *testing.T) {
		config := WorkspaceConfig{
			Workspace: "test",
			Libraries: []WorkspaceLibrary{
				{Name: "a", Deps: []WorkspaceDep{{Lib: "b", From: "global"}}},
				{Name: "b", Deps: []WorkspaceDep{{Lib: "a", From: "global"}}},
			},
		}
		_, err := ScopedRefreshOrder(config, "a", false)
		if err == nil || !strings.Contains(err.Error(), "cycle") {
			t.Fatalf("expected a dependency cycle error, got: %v", err)
		}
	})

	t.Run("chaos__ScopedRefreshOrder__emptyAndMalformedRepoNamesError", func(t *testing.T) {
		config := makeScopedConfig()
		for _, repo := range []string{"", "   ", "../etc/passwd", "core; rm -rf /", "lib:global:core"} {
			if _, err := ScopedRefreshOrder(config, repo, false); err == nil {
				t.Fatalf("expected error for malformed repo name %q, got nil", repo)
			}
		}
	})
}
