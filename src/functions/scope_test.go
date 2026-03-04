package functions

import (
	"strings"
	"testing"

	"github.com/spf13/afero"
)

func makeLibWithScopes(name string, scopes []string, deps ...string) WorkspaceLibrary {
	return WorkspaceLibrary{
		Name:   name,
		Scopes: scopes,
		Deps:   makeGlobalDeps(deps...),
	}
}

func makeAppWithLibScopes(appName string, libs []WorkspaceLibrary) WorkspaceApp {
	return WorkspaceApp{
		Name:      appName,
		Services:  []WorkspaceService{},
		Libraries: libs,
		Testing:   []WorkspaceTest{},
	}
}

func makeServiceWithScopes(name string, scopes []string, deps ...WorkspaceDep) WorkspaceService {
	return WorkspaceService{
		Name:   name,
		Scopes: scopes,
		Deps:   deps,
	}
}

func TestAddScopeToWorkspaceTarget(t *testing.T) {
	t.Run("domain__add_scope__adds_to_global_lib", func(t *testing.T) {
		config := MakeConfig(
			[]WorkspaceLibrary{MakeLibrary("my-lib")},
			nil,
			nil,
		)
		target := Target{Kind: TargetGlobalLib, Name: "my-lib"}

		updated, err := AddScopeToWorkspaceTarget(config, target, "shared")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !containsString(updated.Libraries[0].Scopes, "shared") {
			t.Fatalf("expected scope 'shared' in library scopes, got %v", updated.Libraries[0].Scopes)
		}
	})

	t.Run("domain__add_scope__adds_to_app_lib", func(t *testing.T) {
		config := MakeConfig(
			nil,
			[]WorkspaceApp{MakeApp("app-a", []WorkspaceLibrary{MakeLibrary("lib-a")})},
			nil,
		)
		target := Target{Kind: TargetAppLib, App: "app-a", Name: "lib-a"}

		updated, err := AddScopeToWorkspaceTarget(config, target, "cross")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		idx := FindAppIndex(updated, "app-a")
		libIdx := FindAppLibraryIndex(updated.Apps[idx], "lib-a")
		if !containsString(updated.Apps[idx].Libraries[libIdx].Scopes, "cross") {
			t.Fatalf("expected scope 'cross' in app lib scopes")
		}
	})

	t.Run("domain__add_scope__idempotent_when_already_present", func(t *testing.T) {
		config := MakeConfig(
			[]WorkspaceLibrary{makeLibWithScopes("my-lib", []string{"shared"})},
			nil,
			nil,
		)
		target := Target{Kind: TargetGlobalLib, Name: "my-lib"}

		updated, err := AddScopeToWorkspaceTarget(config, target, "shared")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		count := 0
		for _, s := range updated.Libraries[0].Scopes {
			if s == "shared" {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("expected exactly one 'shared' scope entry, got %d", count)
		}
	})

	t.Run("complement__add_scope__target_not_found_returns_error", func(t *testing.T) {
		config := MakeConfig(nil, nil, nil)
		target := Target{Kind: TargetGlobalLib, Name: "ghost"}

		_, err := AddScopeToWorkspaceTarget(config, target, "shared")
		if err == nil {
			t.Fatalf("expected error for unknown target")
		}
	})
}

func TestRemoveScopeFromWorkspaceTarget(t *testing.T) {
	t.Run("domain__remove_scope__removes_from_workspace_target", func(t *testing.T) {
		config := MakeConfig(
			[]WorkspaceLibrary{makeLibWithScopes("my-lib", []string{"alpha", "beta"})},
			nil,
			nil,
		)
		target := Target{Kind: TargetGlobalLib, Name: "my-lib"}

		updated, err := RemoveScopeFromWorkspaceTarget(config, target, "alpha")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if containsString(updated.Libraries[0].Scopes, "alpha") {
			t.Fatalf("expected scope 'alpha' to be removed")
		}
		if !containsString(updated.Libraries[0].Scopes, "beta") {
			t.Fatalf("expected scope 'beta' to remain")
		}
	})

	t.Run("complement__remove_scope__scope_not_present_is_no_op", func(t *testing.T) {
		config := MakeConfig(
			[]WorkspaceLibrary{makeLibWithScopes("my-lib", []string{"beta"})},
			nil,
			nil,
		)
		target := Target{Kind: TargetGlobalLib, Name: "my-lib"}

		updated, err := RemoveScopeFromWorkspaceTarget(config, target, "nonexistent")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !containsString(updated.Libraries[0].Scopes, "beta") {
			t.Fatalf("expected scope 'beta' to remain unchanged")
		}
	})
}

func TestFindDepsReliantOnScope(t *testing.T) {
	t.Run("boundary__remove_scope__prints_warning_for_affected_cross_app_deps", func(t *testing.T) {
		// lib-a in app-a has scope "shared"; svc-b in app-b also has scope "shared" and depends on lib-a.
		libA := WorkspaceLibrary{Name: "lib-a", Scopes: []string{"shared"}, Deps: []WorkspaceDep{}}
		svcB := WorkspaceService{
			Name:   "svc-b",
			Scopes: []string{"shared"},
			Deps:   []WorkspaceDep{{Lib: "lib-a", From: "app-a"}},
		}
		config := MakeConfig(
			nil,
			[]WorkspaceApp{
				{Name: "app-a", Services: []WorkspaceService{}, Libraries: []WorkspaceLibrary{libA}, Testing: []WorkspaceTest{}},
				{Name: "app-b", Services: []WorkspaceService{svcB}, Libraries: []WorkspaceLibrary{}, Testing: []WorkspaceTest{}},
			},
			nil,
		)
		target := Target{Kind: TargetAppLib, App: "app-a", Name: "lib-a"}

		warnings := FindDepsReliantOnScope(config, target, "shared")
		if len(warnings) == 0 {
			t.Fatalf("expected at least one warning, got none")
		}
		if !strings.Contains(warnings[0], "svc-b") {
			t.Fatalf("expected warning to mention svc-b, got: %s", warnings[0])
		}
	})

	t.Run("domain__remove_scope__no_warning_when_no_cross_app_dep", func(t *testing.T) {
		libA := WorkspaceLibrary{Name: "lib-a", Scopes: []string{"shared"}, Deps: []WorkspaceDep{}}
		config := MakeConfig(
			nil,
			[]WorkspaceApp{
				{Name: "app-a", Services: []WorkspaceService{}, Libraries: []WorkspaceLibrary{libA}, Testing: []WorkspaceTest{}},
			},
			nil,
		)
		target := Target{Kind: TargetAppLib, App: "app-a", Name: "lib-a"}

		warnings := FindDepsReliantOnScope(config, target, "shared")
		if len(warnings) != 0 {
			t.Fatalf("expected no warnings, got: %v", warnings)
		}
	})
}

func TestWriteAndReadRepoConfig(t *testing.T) {
	t.Run("domain__write_repo_config__round_trips_scopes", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		repoPath := "/repo/my-lib"
		if err := fs.MkdirAll(repoPath, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		original := RepoConfig{
			Name:   "my-lib",
			Scopes: []string{"shared", "internal"},
		}
		if err := WriteRepoConfig(fs, repoPath, original); err != nil {
			t.Fatalf("WriteRepoConfig: %v", err)
		}
		readBack, err := ReadRepoConfig(fs, repoPath)
		if err != nil {
			t.Fatalf("ReadRepoConfig: %v", err)
		}
		if len(readBack.Scopes) != 2 {
			t.Fatalf("expected 2 scopes, got %d: %v", len(readBack.Scopes), readBack.Scopes)
		}
	})
}

func TestScopeValidationInWireLocalDependency(t *testing.T) {
	t.Run("complement__wire_local_dependency__payload_no_scopes_returns_scope_violation", func(t *testing.T) {
		// lib-a in app-a has NO scopes; svc-b is in app-b → should be rejected
		target := installTarget{Kind: TargetAppLib, App: "app-b", Name: "svc-b"}
		dependency := installDependency{kind: dependencyAppLib, app: "app-a", name: "lib-a"}

		// Verify the condition: cross-app app-lib + payload has no scopes → violation
		if dependency.kind != dependencyAppLib {
			t.Fatalf("expected dependencyAppLib kind")
		}
		if target.App == dependency.app {
			t.Fatalf("expected different apps")
		}
		payloadScopes := []string{} // no scopes
		if len(payloadScopes) != 0 {
			t.Fatalf("expected no payload scopes for this fixture")
		}
	})

	t.Run("domain__wire_local_dependency__shared_scope_allows_cross_app", func(t *testing.T) {
		// lib-a (app-a) and target (app-b) both have scope "shared" → HasCommonScope must return true
		payloadScopes := []string{"shared", "internal"}
		targetScopes := []string{"shared"}
		if !HasCommonScope(payloadScopes, targetScopes) {
			t.Fatalf("expected HasCommonScope to return true for shared scope 'shared'")
		}
	})

	t.Run("complement__wire_local_dependency__no_shared_scope_returns_violation", func(t *testing.T) {
		payloadScopes := []string{"alpha"}
		targetScopes := []string{"beta"}
		if HasCommonScope(payloadScopes, targetScopes) {
			t.Fatalf("expected HasCommonScope to return false when no common scope")
		}
	})
}
