package functions

import (
	"testing"

	"github.com/spf13/afero"
)

func TestMoveInWorkspaceConfig(t *testing.T) {
	t.Run("domain__move__app_to_app_relocates_library", func(t *testing.T) {
		libA := WorkspaceLibrary{Name: "lib-a", Deps: []WorkspaceDep{}}
		config := MakeConfig(
			nil,
			[]WorkspaceApp{
				{Name: "app-a", Services: []WorkspaceService{}, Libraries: []WorkspaceLibrary{libA}, Testing: []WorkspaceTest{}},
				{Name: "app-b", Services: []WorkspaceService{}, Libraries: []WorkspaceLibrary{}, Testing: []WorkspaceTest{}},
			},
			nil,
		)

		updated, err := MoveInWorkspaceConfig(config, MoveLibraryOptions{
			Library: "lib-a", FromApp: "app-a", ToApp: "app-b",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		appAIdx := FindAppIndex(updated, "app-a")
		if FindAppLibraryIndex(updated.Apps[appAIdx], "lib-a") != -1 {
			t.Fatalf("expected lib-a removed from app-a")
		}
		appBIdx := FindAppIndex(updated, "app-b")
		if FindAppLibraryIndex(updated.Apps[appBIdx], "lib-a") == -1 {
			t.Fatalf("expected lib-a added to app-b")
		}
	})

	t.Run("domain__move__app_to_app_updates_dep_references", func(t *testing.T) {
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

		updated, err := MoveInWorkspaceConfig(config, MoveLibraryOptions{
			Library: "lib-a", FromApp: "app-a", ToApp: "app-b",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		appBIdx := FindAppIndex(updated, "app-b")
		svcIdx := FindServiceIndex(updated.Apps[appBIdx], "svc-b")
		deps := updated.Apps[appBIdx].Services[svcIdx].Deps
		if len(deps) != 1 || deps[0].From != "app-b" {
			t.Fatalf("expected dep From updated to app-b, got %+v", deps)
		}
	})

	t.Run("domain__move__app_to_global_relocates_library", func(t *testing.T) {
		libA := WorkspaceLibrary{Name: "lib-a", Deps: []WorkspaceDep{}}
		config := MakeConfig(
			nil,
			[]WorkspaceApp{
				{Name: "app-a", Services: []WorkspaceService{}, Libraries: []WorkspaceLibrary{libA}, Testing: []WorkspaceTest{}},
			},
			nil,
		)

		updated, err := MoveInWorkspaceConfig(config, MoveLibraryOptions{
			Library: "lib-a", FromApp: "app-a", ToGlobal: true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		appIdx := FindAppIndex(updated, "app-a")
		if FindAppLibraryIndex(updated.Apps[appIdx], "lib-a") != -1 {
			t.Fatalf("expected lib-a removed from app-a")
		}
		if FindGlobalLibraryIndex(updated, "lib-a") == -1 {
			t.Fatalf("expected lib-a added to global libs")
		}
	})

	t.Run("domain__move__app_to_global_updates_dep_references", func(t *testing.T) {
		libA := WorkspaceLibrary{Name: "lib-a", Deps: []WorkspaceDep{}}
		svcA := WorkspaceService{
			Name: "svc-a",
			Deps: []WorkspaceDep{{Lib: "lib-a", From: "app-a"}},
		}
		config := MakeConfig(
			nil,
			[]WorkspaceApp{
				{Name: "app-a", Services: []WorkspaceService{svcA}, Libraries: []WorkspaceLibrary{libA}, Testing: []WorkspaceTest{}},
			},
			nil,
		)

		updated, err := MoveInWorkspaceConfig(config, MoveLibraryOptions{
			Library: "lib-a", FromApp: "app-a", ToGlobal: true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		appIdx := FindAppIndex(updated, "app-a")
		svcIdx := FindServiceIndex(updated.Apps[appIdx], "svc-a")
		deps := updated.Apps[appIdx].Services[svcIdx].Deps
		if len(deps) != 1 || deps[0].From != "global" {
			t.Fatalf("expected dep From updated to global, got %+v", deps)
		}
	})

	t.Run("domain__move__global_to_app_relocates_library", func(t *testing.T) {
		config := MakeConfig(
			[]WorkspaceLibrary{MakeLibrary("lib-a")},
			[]WorkspaceApp{
				{Name: "app-a", Services: []WorkspaceService{}, Libraries: []WorkspaceLibrary{}, Testing: []WorkspaceTest{}},
			},
			nil,
		)

		updated, err := MoveInWorkspaceConfig(config, MoveLibraryOptions{
			Library: "lib-a", FromGlobal: true, ToApp: "app-a",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if FindGlobalLibraryIndex(updated, "lib-a") != -1 {
			t.Fatalf("expected lib-a removed from global libs")
		}
		appIdx := FindAppIndex(updated, "app-a")
		if FindAppLibraryIndex(updated.Apps[appIdx], "lib-a") == -1 {
			t.Fatalf("expected lib-a added to app-a")
		}
	})

	t.Run("domain__move__global_to_app_updates_dep_references", func(t *testing.T) {
		svcA := WorkspaceService{
			Name: "svc-a",
			Deps: []WorkspaceDep{{Lib: "lib-a", From: "global"}},
		}
		config := MakeConfig(
			[]WorkspaceLibrary{MakeLibrary("lib-a")},
			[]WorkspaceApp{
				{Name: "app-a", Services: []WorkspaceService{svcA}, Libraries: []WorkspaceLibrary{}, Testing: []WorkspaceTest{}},
			},
			nil,
		)

		updated, err := MoveInWorkspaceConfig(config, MoveLibraryOptions{
			Library: "lib-a", FromGlobal: true, ToApp: "app-a",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		appIdx := FindAppIndex(updated, "app-a")
		svcIdx := FindServiceIndex(updated.Apps[appIdx], "svc-a")
		deps := updated.Apps[appIdx].Services[svcIdx].Deps
		if len(deps) != 1 || deps[0].From != "app-a" {
			t.Fatalf("expected dep From updated to app-a, got %+v", deps)
		}
	})

	t.Run("complement__move__name_conflict_at_destination_returns_error", func(t *testing.T) {
		config := MakeConfig(
			[]WorkspaceLibrary{MakeLibrary("lib-a")},
			[]WorkspaceApp{
				{Name: "app-a", Services: []WorkspaceService{}, Libraries: []WorkspaceLibrary{{Name: "lib-a", Deps: []WorkspaceDep{}}}, Testing: []WorkspaceTest{}},
			},
			nil,
		)

		_, err := MoveInWorkspaceConfig(config, MoveLibraryOptions{
			Library: "lib-a", FromGlobal: true, ToApp: "app-a",
		})
		if err == nil {
			t.Fatalf("expected name conflict error")
		}
	})

	t.Run("complement__move__source_not_found_returns_error", func(t *testing.T) {
		config := MakeConfig(nil, nil, nil)

		_, err := MoveInWorkspaceConfig(config, MoveLibraryOptions{
			Library: "ghost", FromGlobal: true, ToApp: "app-a",
		})
		if err == nil {
			t.Fatalf("expected source not found error")
		}
	})

	t.Run("complement__move__dest_app_not_found_returns_error", func(t *testing.T) {
		config := MakeConfig(
			[]WorkspaceLibrary{MakeLibrary("lib-a")},
			nil,
			nil,
		)

		_, err := MoveInWorkspaceConfig(config, MoveLibraryOptions{
			Library: "lib-a", FromGlobal: true, ToApp: "no-such-app",
		})
		if err == nil {
			t.Fatalf("expected destination app not found error")
		}
	})

	t.Run("complement__move__source_app_not_found_returns_error", func(t *testing.T) {
		config := MakeConfig(nil, nil, nil)

		_, err := MoveInWorkspaceConfig(config, MoveLibraryOptions{
			Library: "lib-a", FromApp: "no-such-app", ToGlobal: true,
		})
		if err == nil {
			t.Fatalf("expected source app not found error")
		}
	})

	t.Run("domain__move__global_to_app_name_conflict_returns_error", func(t *testing.T) {
		libA := WorkspaceLibrary{Name: "lib-a", Deps: []WorkspaceDep{}}
		config := MakeConfig(
			[]WorkspaceLibrary{MakeLibrary("lib-a")},
			[]WorkspaceApp{
				{Name: "app-a", Services: []WorkspaceService{}, Libraries: []WorkspaceLibrary{libA}, Testing: []WorkspaceTest{}},
			},
			nil,
		)

		_, err := MoveInWorkspaceConfig(config, MoveLibraryOptions{
			Library: "lib-a", FromGlobal: true, ToApp: "app-a",
		})
		if err == nil {
			t.Fatalf("expected name conflict error at destination")
		}
	})

	t.Run("domain__move__app_to_global_name_conflict_returns_error", func(t *testing.T) {
		libA := WorkspaceLibrary{Name: "lib-a", Deps: []WorkspaceDep{}}
		config := MakeConfig(
			[]WorkspaceLibrary{MakeLibrary("lib-a")},
			[]WorkspaceApp{
				{Name: "app-a", Services: []WorkspaceService{}, Libraries: []WorkspaceLibrary{libA}, Testing: []WorkspaceTest{}},
			},
			nil,
		)

		_, err := MoveInWorkspaceConfig(config, MoveLibraryOptions{
			Library: "lib-a", FromApp: "app-a", ToGlobal: true,
		})
		if err == nil {
			t.Fatalf("expected name conflict error at global destination")
		}
	})
}

func TestFindMoveScopeWarnings(t *testing.T) {
	t.Run("domain__move__global_to_app_warns_on_scope_violation", func(t *testing.T) {
		// lib-a moved from global to app-a. svc-b in app-b depends on it but has no shared scope.
		libA := WorkspaceLibrary{Name: "lib-a", Deps: []WorkspaceDep{}}
		svcB := WorkspaceService{
			Name: "svc-b",
			Deps: []WorkspaceDep{{Lib: "lib-a", From: "app-a"}},
		}
		config := MakeConfig(
			nil,
			[]WorkspaceApp{
				{Name: "app-a", Services: []WorkspaceService{}, Libraries: []WorkspaceLibrary{libA}, Testing: []WorkspaceTest{}},
				{Name: "app-b", Services: []WorkspaceService{svcB}, Libraries: []WorkspaceLibrary{}, Testing: []WorkspaceTest{}},
			},
			nil,
		)

		warnings := FindMoveScopeWarnings(config, MoveLibraryOptions{
			Library: "lib-a", FromGlobal: true, ToApp: "app-a",
		})
		if len(warnings) == 0 {
			t.Fatalf("expected scope violation warning")
		}
	})

	t.Run("domain__move__app_to_app_warns_on_scope_violation", func(t *testing.T) {
		// lib-a moved from app-a to app-c. svc-b in app-b depends on it but has no shared scope with app-c.
		libA := WorkspaceLibrary{Name: "lib-a", Deps: []WorkspaceDep{}}
		svcB := WorkspaceService{
			Name: "svc-b",
			Deps: []WorkspaceDep{{Lib: "lib-a", From: "app-c"}},
		}
		config := MakeConfig(
			nil,
			[]WorkspaceApp{
				{Name: "app-a", Services: []WorkspaceService{}, Libraries: []WorkspaceLibrary{}, Testing: []WorkspaceTest{}},
				{Name: "app-b", Services: []WorkspaceService{svcB}, Libraries: []WorkspaceLibrary{}, Testing: []WorkspaceTest{}},
				{Name: "app-c", Services: []WorkspaceService{}, Libraries: []WorkspaceLibrary{libA}, Testing: []WorkspaceTest{}},
			},
			nil,
		)

		warnings := FindMoveScopeWarnings(config, MoveLibraryOptions{
			Library: "lib-a", FromApp: "app-a", ToApp: "app-c",
		})
		if len(warnings) == 0 {
			t.Fatalf("expected scope violation warning")
		}
	})

	t.Run("complement__move__no_warning_when_scopes_match", func(t *testing.T) {
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

		warnings := FindMoveScopeWarnings(config, MoveLibraryOptions{
			Library: "lib-a", FromGlobal: true, ToApp: "app-a",
		})
		if len(warnings) != 0 {
			t.Fatalf("expected no warnings when scopes match, got %v", warnings)
		}
	})

	t.Run("complement__move__no_warning_when_to_global", func(t *testing.T) {
		libA := WorkspaceLibrary{Name: "lib-a", Deps: []WorkspaceDep{}}
		svcB := WorkspaceService{
			Name: "svc-b",
			Deps: []WorkspaceDep{{Lib: "lib-a", From: "global"}},
		}
		config := MakeConfig(
			[]WorkspaceLibrary{libA},
			[]WorkspaceApp{
				{Name: "app-b", Services: []WorkspaceService{svcB}, Libraries: []WorkspaceLibrary{}, Testing: []WorkspaceTest{}},
			},
			nil,
		)

		warnings := FindMoveScopeWarnings(config, MoveLibraryOptions{
			Library: "lib-a", FromApp: "app-a", ToGlobal: true,
		})
		if len(warnings) != 0 {
			t.Fatalf("expected no warnings for move to global, got %v", warnings)
		}
	})
}

func TestMoveHashFiles(t *testing.T) {
	t.Run("domain__move_hash_files__global_to_app", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/root"
		oldBuild := MakeHashPath(root, "libs", "global", "lib-a")
		oldCheck := MakeCheckHashPath(root, "libs", "global", "lib-a")

		if err := WriteHashFile(fs, oldBuild, "abc"); err != nil {
			t.Fatalf("setup build hash: %v", err)
		}
		if err := WriteHashFile(fs, oldCheck, "def"); err != nil {
			t.Fatalf("setup check hash: %v", err)
		}

		opts := MoveLibraryOptions{Library: "lib-a", FromGlobal: true, ToApp: "app-a"}
		if err := MoveHashFiles(fs, root, opts); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		newBuild := MakeHashPath(root, "libs", "app-a", "lib-a")
		newCheck := MakeCheckHashPath(root, "libs", "app-a", "lib-a")
		if _, ok, _ := ReadHashFile(fs, newBuild); !ok {
			t.Fatalf("expected new build hash at %s", newBuild)
		}
		if _, ok, _ := ReadHashFile(fs, newCheck); !ok {
			t.Fatalf("expected new check hash at %s", newCheck)
		}
		if _, ok, _ := ReadHashFile(fs, oldBuild); ok {
			t.Fatalf("expected old build hash removed")
		}
		if _, ok, _ := ReadHashFile(fs, oldCheck); ok {
			t.Fatalf("expected old check hash removed")
		}
	})

	t.Run("domain__move_hash_files__app_to_global", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/root"
		oldBuild := MakeHashPath(root, "libs", "app-a", "lib-a")

		if err := WriteHashFile(fs, oldBuild, "xyz"); err != nil {
			t.Fatalf("setup: %v", err)
		}

		opts := MoveLibraryOptions{Library: "lib-a", FromApp: "app-a", ToGlobal: true}
		if err := MoveHashFiles(fs, root, opts); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		newBuild := MakeHashPath(root, "libs", "global", "lib-a")
		if _, ok, _ := ReadHashFile(fs, newBuild); !ok {
			t.Fatalf("expected new build hash at %s", newBuild)
		}
		if _, ok, _ := ReadHashFile(fs, oldBuild); ok {
			t.Fatalf("expected old build hash removed")
		}
	})

	t.Run("domain__move_hash_files__app_to_app", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/root"
		oldBuild := MakeHashPath(root, "libs", "app-a", "lib-a")
		oldCheck := MakeCheckHashPath(root, "libs", "app-a", "lib-a")

		if err := WriteHashFile(fs, oldBuild, "h1"); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := WriteHashFile(fs, oldCheck, "h2"); err != nil {
			t.Fatalf("setup: %v", err)
		}

		opts := MoveLibraryOptions{Library: "lib-a", FromApp: "app-a", ToApp: "app-b"}
		if err := MoveHashFiles(fs, root, opts); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		newBuild := MakeHashPath(root, "libs", "app-b", "lib-a")
		newCheck := MakeCheckHashPath(root, "libs", "app-b", "lib-a")
		if _, ok, _ := ReadHashFile(fs, newBuild); !ok {
			t.Fatalf("expected new build hash at %s", newBuild)
		}
		if _, ok, _ := ReadHashFile(fs, newCheck); !ok {
			t.Fatalf("expected new check hash at %s", newCheck)
		}
	})

	t.Run("complement__move_hash_files__missing_hashes_are_skipped", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/root"

		opts := MoveLibraryOptions{Library: "lib-a", FromGlobal: true, ToApp: "app-a"}
		if err := MoveHashFiles(fs, root, opts); err != nil {
			t.Fatalf("expected no error for missing hash files, got: %v", err)
		}
	})
}
