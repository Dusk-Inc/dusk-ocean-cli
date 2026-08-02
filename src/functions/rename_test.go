package functions

import (
	"testing"

	"github.com/spf13/afero"
)

func TestRenameInWorkspaceConfig(t *testing.T) {
	t.Run("domain__rename__updates_global_lib_name_in_workspace", func(t *testing.T) {
		config := MakeConfig(
			[]WorkspaceLibrary{MakeLibrary("lib-a")},
			nil,
			nil,
		)
		target := Target{Kind: TargetGlobalLib, Name: "lib-a"}

		updated, err := RenameInWorkspaceConfig(config, target, "lib-a", "lib-b")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if FindGlobalLibraryIndex(updated, "lib-a") != -1 {
			t.Fatalf("expected old name 'lib-a' to be removed")
		}
		if FindGlobalLibraryIndex(updated, "lib-b") == -1 {
			t.Fatalf("expected new name 'lib-b' to exist")
		}
	})

	t.Run("domain__rename__updates_global_lib_dep_references", func(t *testing.T) {
		config := MakeConfig(
			[]WorkspaceLibrary{
				MakeLibrary("lib-a"),
				MakeLibrary("lib-b", "lib-a"),
			},
			nil,
			nil,
		)
		target := Target{Kind: TargetGlobalLib, Name: "lib-a"}

		updated, err := RenameInWorkspaceConfig(config, target, "lib-a", "lib-x")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		idx := FindGlobalLibraryIndex(updated, "lib-b")
		if idx == -1 {
			t.Fatalf("lib-b missing after rename")
		}
		deps := updated.Libraries[idx].Deps
		if len(deps) != 1 || deps[0].Lib != "lib-x" || deps[0].From != "global" {
			t.Fatalf("expected dep updated to lib-x/global, got %+v", deps)
		}
	})

	t.Run("domain__rename__updates_project_dep_references_to_global_lib", func(t *testing.T) {
		config := MakeConfig(
			[]WorkspaceLibrary{MakeLibrary("lib-a")},
			nil,
			[]WorkspaceProject{MakeProject("proj", "lib-a")},
		)
		target := Target{Kind: TargetGlobalLib, Name: "lib-a"}

		updated, err := RenameInWorkspaceConfig(config, target, "lib-a", "lib-x")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		pidx := FindProjectIndex(updated, "proj")
		if pidx == -1 {
			t.Fatalf("project 'proj' missing after rename")
		}
		deps := updated.Projects[pidx].Deps
		if len(deps) != 1 || deps[0].Lib != "lib-x" {
			t.Fatalf("expected project dep updated to lib-x, got %+v", deps)
		}
	})

	t.Run("domain__rename__updates_app_lib_name_in_workspace", func(t *testing.T) {
		config := MakeConfig(
			nil,
			[]WorkspaceApp{MakeApp("app-a", []WorkspaceLibrary{MakeLibrary("lib-a")})},
			nil,
		)
		target := Target{Kind: TargetAppLib, App: "app-a", Name: "lib-a"}

		updated, err := RenameInWorkspaceConfig(config, target, "lib-a", "lib-b")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		appIdx := FindAppIndex(updated, "app-a")
		if FindAppLibraryIndex(updated.Apps[appIdx], "lib-a") != -1 {
			t.Fatalf("expected old name to be removed")
		}
		if FindAppLibraryIndex(updated.Apps[appIdx], "lib-b") == -1 {
			t.Fatalf("expected new name to be present")
		}
	})

	t.Run("domain__rename__updates_app_lib_dep_references_within_app", func(t *testing.T) {
		libA := WorkspaceLibrary{Name: "lib-a", Deps: []WorkspaceDep{}}
		svcA := WorkspaceService{
			Name: "svc-a",
			Deps: []WorkspaceDep{{Lib: "lib-a", From: "app-a"}},
		}
		config := MakeConfig(
			nil,
			[]WorkspaceApp{{
				Name:      "app-a",
				Services:  []WorkspaceService{svcA},
				Libraries: []WorkspaceLibrary{libA},
				Testing:   []WorkspaceTest{},
			}},
			nil,
		)
		target := Target{Kind: TargetAppLib, App: "app-a", Name: "lib-a"}

		updated, err := RenameInWorkspaceConfig(config, target, "lib-a", "lib-x")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		appIdx := FindAppIndex(updated, "app-a")
		svcIdx := FindServiceIndex(updated.Apps[appIdx], "svc-a")
		deps := updated.Apps[appIdx].Services[svcIdx].Deps
		if len(deps) != 1 || deps[0].Lib != "lib-x" || deps[0].From != "app-a" {
			t.Fatalf("expected dep updated to lib-x/app-a, got %+v", deps)
		}
	})

	t.Run("domain__rename__updates_project_name_in_workspace", func(t *testing.T) {
		config := MakeConfig(nil, nil, []WorkspaceProject{MakeProject("proj-a")})
		target := Target{Kind: TargetProject, Name: "proj-a"}

		updated, err := RenameInWorkspaceConfig(config, target, "proj-a", "proj-b")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if FindProjectIndex(updated, "proj-a") != -1 {
			t.Fatalf("expected old project name to be removed")
		}
		if FindProjectIndex(updated, "proj-b") == -1 {
			t.Fatalf("expected new project name to be present")
		}
	})

	t.Run("domain__rename__updates_service_dep_references_to_renamed_project", func(t *testing.T) {
		svcA := WorkspaceService{
			Name: "svc-a",
			Deps: []WorkspaceDep{{Lib: "proj-a", From: "project"}},
		}
		config := MakeConfig(
			nil,
			[]WorkspaceApp{{
				Name:      "app-a",
				Services:  []WorkspaceService{svcA},
				Libraries: []WorkspaceLibrary{},
				Testing:   []WorkspaceTest{},
			}},
			[]WorkspaceProject{{Name: "proj-a", Deps: []WorkspaceDep{}}},
		)
		target := Target{Kind: TargetProject, Name: "proj-a"}

		updated, err := RenameInWorkspaceConfig(config, target, "proj-a", "proj-b")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		appIdx := FindAppIndex(updated, "app-a")
		svcIdx := FindServiceIndex(updated.Apps[appIdx], "svc-a")
		deps := updated.Apps[appIdx].Services[svcIdx].Deps
		if len(deps) != 1 || deps[0].Lib != "proj-b" || deps[0].From != "project" {
			t.Fatalf("expected dep updated to proj-b/project, got %+v", deps)
		}
	})

	t.Run("domain__rename__updates_service_name_in_workspace", func(t *testing.T) {
		svcA := WorkspaceService{Name: "svc-a", Deps: []WorkspaceDep{}}
		config := MakeConfig(
			nil,
			[]WorkspaceApp{{
				Name:      "app-a",
				Services:  []WorkspaceService{svcA},
				Libraries: []WorkspaceLibrary{},
				Testing:   []WorkspaceTest{},
			}},
			nil,
		)
		target := Target{Kind: TargetService, App: "app-a", Name: "svc-a"}

		updated, err := RenameInWorkspaceConfig(config, target, "svc-a", "svc-b")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		appIdx := FindAppIndex(updated, "app-a")
		if FindServiceIndex(updated.Apps[appIdx], "svc-a") != -1 {
			t.Fatalf("expected old service name to be removed")
		}
		if FindServiceIndex(updated.Apps[appIdx], "svc-b") == -1 {
			t.Fatalf("expected new service name to be present")
		}
	})

	t.Run("domain__rename__updates_test_name_in_workspace", func(t *testing.T) {
		testA := WorkspaceTest{Name: "suite-a", Deps: []WorkspaceDep{}}
		config := MakeConfig(
			nil,
			[]WorkspaceApp{{
				Name:      "app-a",
				Services:  []WorkspaceService{},
				Libraries: []WorkspaceLibrary{},
				Testing:   []WorkspaceTest{testA},
			}},
			nil,
		)
		target := Target{Kind: TargetTest, App: "app-a", Name: "suite-a"}

		updated, err := RenameInWorkspaceConfig(config, target, "suite-a", "suite-b")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		appIdx := FindAppIndex(updated, "app-a")
		if FindAppTestIndex(updated.Apps[appIdx], "suite-a") != -1 {
			t.Fatalf("expected old test name to be removed")
		}
		if FindAppTestIndex(updated.Apps[appIdx], "suite-b") == -1 {
			t.Fatalf("expected new test name to be present")
		}
	})

	t.Run("complement__rename__global_lib_not_found_returns_error", func(t *testing.T) {
		config := MakeConfig(nil, nil, nil)
		target := Target{Kind: TargetGlobalLib, Name: "ghost"}

		_, err := RenameInWorkspaceConfig(config, target, "ghost", "new-name")
		if err == nil {
			t.Fatalf("expected error for unknown global lib")
		}
	})

	t.Run("complement__rename__app_lib_not_found_returns_error", func(t *testing.T) {
		config := MakeConfig(nil, []WorkspaceApp{MakeApp("app-a", nil)}, nil)
		target := Target{Kind: TargetAppLib, App: "app-a", Name: "ghost"}

		_, err := RenameInWorkspaceConfig(config, target, "ghost", "new-name")
		if err == nil {
			t.Fatalf("expected error for unknown app lib")
		}
	})

	t.Run("complement__rename__project_not_found_returns_error", func(t *testing.T) {
		config := MakeConfig(nil, nil, nil)
		target := Target{Kind: TargetProject, Name: "ghost"}

		_, err := RenameInWorkspaceConfig(config, target, "ghost", "new-name")
		if err == nil {
			t.Fatalf("expected error for unknown project")
		}
	})

	t.Run("boundary__rename__app_lib_dep_with_same_name_in_other_app_not_affected", func(t *testing.T) {

		libAInAppA := WorkspaceLibrary{Name: "lib-a", Deps: []WorkspaceDep{}}
		libAInAppB := WorkspaceLibrary{Name: "lib-a", Deps: []WorkspaceDep{}}
		svcB := WorkspaceService{
			Name: "svc-b",
			Deps: []WorkspaceDep{{Lib: "lib-a", From: "app-b"}},
		}
		config := MakeConfig(
			nil,
			[]WorkspaceApp{
				{Name: "app-a", Services: []WorkspaceService{}, Libraries: []WorkspaceLibrary{libAInAppA}, Testing: []WorkspaceTest{}},
				{Name: "app-b", Services: []WorkspaceService{svcB}, Libraries: []WorkspaceLibrary{libAInAppB}, Testing: []WorkspaceTest{}},
			},
			nil,
		)
		target := Target{Kind: TargetAppLib, App: "app-a", Name: "lib-a"}

		updated, err := RenameInWorkspaceConfig(config, target, "lib-a", "lib-x")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		appBIdx := FindAppIndex(updated, "app-b")
		svcIdx := FindServiceIndex(updated.Apps[appBIdx], "svc-b")
		deps := updated.Apps[appBIdx].Services[svcIdx].Deps
		if len(deps) != 1 || deps[0].Lib != "lib-a" || deps[0].From != "app-b" {
			t.Fatalf("expected app-b dep to be unaffected, got %+v", deps)
		}
	})

	t.Run("boundary__rename__cross_app_dep_with_matching_from_is_updated", func(t *testing.T) {

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

		updated, err := RenameInWorkspaceConfig(config, target, "lib-a", "lib-x")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		appBIdx := FindAppIndex(updated, "app-b")
		svcIdx := FindServiceIndex(updated.Apps[appBIdx], "svc-b")
		deps := updated.Apps[appBIdx].Services[svcIdx].Deps
		if len(deps) != 1 || deps[0].Lib != "lib-x" || deps[0].From != "app-a" {
			t.Fatalf("expected cross-app dep to be updated, got %+v", deps)
		}
	})
}

func TestRenameHashFiles(t *testing.T) {
	t.Run("domain__rename_hash_files__moves_build_and_check_hashes_for_global_lib", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/root"
		oldBuild := MakeHashPath(root, "libs", "global", "lib-a")
		oldCheck := MakeCheckHashPath(root, "libs", "global", "lib-a")
		newBuild := MakeHashPath(root, "libs", "global", "lib-x")
		newCheck := MakeCheckHashPath(root, "libs", "global", "lib-x")

		if err := WriteHashFile(fs, oldBuild, "abc"); err != nil {
			t.Fatalf("setup build hash: %v", err)
		}
		if err := WriteHashFile(fs, oldCheck, "def"); err != nil {
			t.Fatalf("setup check hash: %v", err)
		}

		target := Target{Kind: TargetGlobalLib, Name: "lib-a"}
		if err := RenameHashFiles(fs, root, target, "lib-a", "lib-x"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, ok, _ := ReadHashFile(fs, newBuild); !ok {
			t.Fatalf("expected new build hash to exist at %s", newBuild)
		}
		if _, ok, _ := ReadHashFile(fs, newCheck); !ok {
			t.Fatalf("expected new check hash to exist at %s", newCheck)
		}
		if _, ok, _ := ReadHashFile(fs, oldBuild); ok {
			t.Fatalf("expected old build hash to be removed")
		}
		if _, ok, _ := ReadHashFile(fs, oldCheck); ok {
			t.Fatalf("expected old check hash to be removed")
		}
	})

	t.Run("complement__rename_hash_files__missing_hashes_are_skipped", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/root"
		target := Target{Kind: TargetGlobalLib, Name: "lib-a"}

		if err := RenameHashFiles(fs, root, target, "lib-a", "lib-x"); err != nil {
			t.Fatalf("expected no error for missing hash files, got: %v", err)
		}
	})

	t.Run("domain__rename_hash_files__moves_hashes_for_app_lib", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/root"
		oldBuild := MakeHashPath(root, "libs", "app-a", "lib-a")
		newBuild := MakeHashPath(root, "libs", "app-a", "lib-x")

		if err := WriteHashFile(fs, oldBuild, "abc"); err != nil {
			t.Fatalf("setup: %v", err)
		}

		target := Target{Kind: TargetAppLib, App: "app-a", Name: "lib-a"}
		if err := RenameHashFiles(fs, root, target, "lib-a", "lib-x"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok, _ := ReadHashFile(fs, newBuild); !ok {
			t.Fatalf("expected new hash to exist at %s", newBuild)
		}
	})

	t.Run("domain__rename_hash_files__moves_hashes_for_service", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/root"
		oldBuild := MakeHashPath(root, "services", "app-a", "svc-a")
		newBuild := MakeHashPath(root, "services", "app-a", "svc-b")

		if err := WriteHashFile(fs, oldBuild, "hash1"); err != nil {
			t.Fatalf("setup: %v", err)
		}

		target := Target{Kind: TargetService, App: "app-a", Name: "svc-a"}
		if err := RenameHashFiles(fs, root, target, "svc-a", "svc-b"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok, _ := ReadHashFile(fs, newBuild); !ok {
			t.Fatalf("expected new service hash to exist")
		}
	})

	t.Run("domain__rename_hash_files__moves_hashes_for_project", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/root"
		oldBuild := MakeHashPath(root, "projects", "global", "proj-a")
		newBuild := MakeHashPath(root, "projects", "global", "proj-x")

		if err := WriteHashFile(fs, oldBuild, "xyz"); err != nil {
			t.Fatalf("setup: %v", err)
		}

		target := Target{Kind: TargetProject, Name: "proj-a"}
		if err := RenameHashFiles(fs, root, target, "proj-a", "proj-x"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok, _ := ReadHashFile(fs, newBuild); !ok {
			t.Fatalf("expected new project hash to exist")
		}
	})

	t.Run("domain__rename_hash_files__moves_hashes_for_app_project", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/root"
		oldBuild := MakeHashPath(root, "projects", "app-a", "cli-a")
		newBuild := MakeHashPath(root, "projects", "app-a", "cli-b")

		if err := WriteHashFile(fs, oldBuild, "xyz"); err != nil {
			t.Fatalf("setup: %v", err)
		}

		target := Target{Kind: TargetAppProject, App: "app-a", Name: "cli-a"}
		if err := RenameHashFiles(fs, root, target, "cli-a", "cli-b"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok, _ := ReadHashFile(fs, newBuild); !ok {
			t.Fatalf("expected new app project hash to exist")
		}
	})

	t.Run("domain__rename_hash_files__moves_hashes_for_test", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/root"
		oldBuild := MakeHashPath(root, "tests", "app-a", "suite-a")
		newBuild := MakeHashPath(root, "tests", "app-a", "suite-b")

		if err := WriteHashFile(fs, oldBuild, "testhash"); err != nil {
			t.Fatalf("setup: %v", err)
		}

		target := Target{Kind: TargetTest, App: "app-a", Name: "suite-a"}
		if err := RenameHashFiles(fs, root, target, "suite-a", "suite-b"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok, _ := ReadHashFile(fs, newBuild); !ok {
			t.Fatalf("expected new test hash to exist")
		}
	})
}

func TestRenameRepoValidation(t *testing.T) {
	t.Run("complement__rename_repo__target_not_found_when_config_empty", func(t *testing.T) {
		config := MakeConfig(nil, nil, nil)
		_, err := ResolveTargetByName(config, "/root", "ghost")
		if err == nil {
			t.Fatalf("expected error for unknown target 'ghost'")
		}
	})

	t.Run("complement__rename_repo__new_name_conflict_detected_when_name_exists", func(t *testing.T) {
		config := MakeConfig(
			[]WorkspaceLibrary{MakeLibrary("lib-a"), MakeLibrary("lib-b")},
			nil,
			nil,
		)

		_, err := ResolveTargetByName(config, "/root", "lib-b")
		if err != nil {
			t.Fatalf("expected lib-b to be found (conflict target): %v", err)
		}
	})

	t.Run("complement__rename_repo__new_name_no_conflict_when_name_absent", func(t *testing.T) {
		config := MakeConfig(
			[]WorkspaceLibrary{MakeLibrary("lib-a")},
			nil,
			nil,
		)

		_, err := ResolveTargetByName(config, "/root", "lib-x")
		if err == nil {
			t.Fatalf("expected error (not found) for 'lib-x', meaning no conflict")
		}
	})
}
