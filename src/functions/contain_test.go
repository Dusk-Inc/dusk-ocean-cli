package functions

import (
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
)

// --- helpers ---

func makeServiceInApp(appName string, serviceName string, deps ...WorkspaceDep) WorkspaceApp {
	return WorkspaceApp{
		Name: appName,
		Services: []WorkspaceService{
			{Name: serviceName, Deps: deps},
		},
		Libraries: []WorkspaceLibrary{},
		Testing:   []WorkspaceTest{},
	}
}

// --- ResolveContainTarget ---

func TestResolveContainTarget(t *testing.T) {
	t.Run("domain__resolve__single_service_match_returns_correct_app_and_service", func(t *testing.T) {
		config := MakeConfig(
			nil,
			[]WorkspaceApp{
				makeServiceInApp("app-a", "svc-x"),
				makeServiceInApp("app-b", "svc-y"),
			},
			nil,
		)

		app, svc, err := ResolveContainTarget(config, "", "svc-x")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if app != "app-a" || svc != "svc-x" {
			t.Fatalf("expected app-a/svc-x, got %s/%s", app, svc)
		}
	})

	t.Run("domain__resolve__explicit_app_resolves_directly", func(t *testing.T) {
		config := MakeConfig(
			nil,
			[]WorkspaceApp{
				makeServiceInApp("app-a", "svc-x"),
				makeServiceInApp("app-b", "svc-x"),
			},
			nil,
		)

		app, svc, err := ResolveContainTarget(config, "app-b", "svc-x")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if app != "app-b" || svc != "svc-x" {
			t.Fatalf("expected app-b/svc-x, got %s/%s", app, svc)
		}
	})

	t.Run("complement__resolve__service_not_found_returns_error", func(t *testing.T) {
		config := MakeConfig(nil, []WorkspaceApp{makeServiceInApp("app-a", "svc-a")}, nil)

		_, _, err := ResolveContainTarget(config, "", "ghost")
		if err == nil {
			t.Fatalf("expected error for unknown service")
		}
	})

	t.Run("complement__resolve__ambiguous_service_returns_error", func(t *testing.T) {
		config := MakeConfig(
			nil,
			[]WorkspaceApp{
				makeServiceInApp("app-a", "svc-x"),
				makeServiceInApp("app-b", "svc-x"),
			},
			nil,
		)

		_, _, err := ResolveContainTarget(config, "", "svc-x")
		if err == nil {
			t.Fatalf("expected ambiguity error")
		}
		if !strings.Contains(err.Error(), "ambiguous") {
			t.Fatalf("expected 'ambiguous' in error message, got: %v", err)
		}
	})

	t.Run("complement__resolve__explicit_app_not_found_returns_error", func(t *testing.T) {
		config := MakeConfig(nil, []WorkspaceApp{makeServiceInApp("app-a", "svc-a")}, nil)

		_, _, err := ResolveContainTarget(config, "ghost-app", "svc-a")
		if err == nil {
			t.Fatalf("expected error for unknown app")
		}
	})
}

// --- ReadOceanIgnorePatterns ---

func TestReadOceanIgnorePatterns(t *testing.T) {
	t.Run("domain__read_oceanignore__returns_patterns_from_file", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/root"
		content := "node_modules/\n# comment\n.cache/\n"
		if err := afero.WriteFile(fs, filepath.Join(root, ".oceanignore"), []byte(content), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}

		patterns, found := ReadOceanIgnorePatterns(fs, root)
		if !found {
			t.Fatalf("expected file to be found")
		}
		if len(patterns) != 2 {
			t.Fatalf("expected 2 patterns, got %d: %v", len(patterns), patterns)
		}
		if patterns[0] != "node_modules/" || patterns[1] != ".cache/" {
			t.Fatalf("unexpected patterns: %v", patterns)
		}
	})

	t.Run("complement__read_oceanignore__missing_file_returns_not_found", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		patterns, found := ReadOceanIgnorePatterns(fs, "/root")
		if found {
			t.Fatalf("expected file to not be found")
		}
		if len(patterns) != 0 {
			t.Fatalf("expected nil patterns, got %v", patterns)
		}
	})
}

// --- ReadOceanIncludePaths ---

func TestReadOceanIncludePaths(t *testing.T) {
	t.Run("domain__read_oceaninclude__returns_paths_from_file", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/root"
		content := "pnpm-workspace.yaml\n# comment\npackage.json\n"
		if err := afero.WriteFile(fs, filepath.Join(root, ".oceaninclude"), []byte(content), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}

		paths, found := ReadOceanIncludePaths(fs, root)
		if !found {
			t.Fatalf("expected file to be found")
		}
		if len(paths) != 2 {
			t.Fatalf("expected 2 paths, got %d: %v", len(paths), paths)
		}
		if paths[0] != "pnpm-workspace.yaml" || paths[1] != "package.json" {
			t.Fatalf("unexpected paths: %v", paths)
		}
	})

	t.Run("complement__read_oceaninclude__missing_file_returns_not_found", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		paths, found := ReadOceanIncludePaths(fs, "/root")
		if found {
			t.Fatalf("expected file to not be found")
		}
		if len(paths) != 0 {
			t.Fatalf("expected nil paths, got %v", paths)
		}
	})
}

// --- StageServiceBuildContext ---

func TestStageServiceBuildContext(t *testing.T) {
	t.Run("domain__stage__service_only_staged_at_correct_relative_path", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/workspace"

		// Seed workspace config.
		config := MakeConfig(nil, []WorkspaceApp{makeServiceInApp("app-a", "svc-a")}, nil)
		if err := writeTestWorkspaceConfig(fs, root, config); err != nil {
			t.Fatalf("setup config: %v", err)
		}

		// Seed service source directory.
		svcPath := filepath.Join(root, "repos", "apps", "app-a", "services", "svc-a")
		if err := afero.WriteFile(fs, filepath.Join(svcPath, "main.go"), []byte("package main"), 0o644); err != nil {
			t.Fatalf("setup svc: %v", err)
		}

		stagingPath, err := StageServiceBuildContext(fs, root, config, "app-a", "svc-a", io.Discard)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expectedFile := filepath.Join(stagingPath, "repos", "apps", "app-a", "services", "svc-a", "main.go")
		if _, err := fs.Stat(expectedFile); err != nil {
			t.Fatalf("expected staged file at %s: %v", expectedFile, err)
		}
	})

	t.Run("domain__stage__global_lib_dep_staged_at_correct_relative_path", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/workspace"

		libA := WorkspaceLibrary{Name: "lib-a", Deps: []WorkspaceDep{}}
		svcA := WorkspaceService{
			Name: "svc-a",
			Deps: []WorkspaceDep{{Lib: "lib-a", From: "global"}},
		}
		config := MakeConfig(
			[]WorkspaceLibrary{libA},
			[]WorkspaceApp{{
				Name:      "app-a",
				Services:  []WorkspaceService{svcA},
				Libraries: []WorkspaceLibrary{},
				Testing:   []WorkspaceTest{},
			}},
			nil,
		)
		if err := writeTestWorkspaceConfig(fs, root, config); err != nil {
			t.Fatalf("setup config: %v", err)
		}

		// Seed source directories.
		svcPath := filepath.Join(root, "repos", "apps", "app-a", "services", "svc-a")
		libPath := filepath.Join(root, "repos", "libs", "lib-a")
		if err := afero.WriteFile(fs, filepath.Join(svcPath, "main.go"), []byte("svc"), 0o644); err != nil {
			t.Fatalf("setup svc: %v", err)
		}
		if err := afero.WriteFile(fs, filepath.Join(libPath, "lib.go"), []byte("lib"), 0o644); err != nil {
			t.Fatalf("setup lib: %v", err)
		}

		stagingPath, err := StageServiceBuildContext(fs, root, config, "app-a", "svc-a", io.Discard)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expectedSvc := filepath.Join(stagingPath, "repos", "apps", "app-a", "services", "svc-a", "main.go")
		expectedLib := filepath.Join(stagingPath, "repos", "libs", "lib-a", "lib.go")
		if _, err := fs.Stat(expectedSvc); err != nil {
			t.Fatalf("expected staged service file: %v", err)
		}
		if _, err := fs.Stat(expectedLib); err != nil {
			t.Fatalf("expected staged lib file: %v", err)
		}
	})

	t.Run("domain__stage__respects_oceanignore_excludes_node_modules", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/workspace"

		config := MakeConfig(nil, []WorkspaceApp{makeServiceInApp("app-a", "svc-a")}, nil)
		if err := writeTestWorkspaceConfig(fs, root, config); err != nil {
			t.Fatalf("setup config: %v", err)
		}

		svcPath := filepath.Join(root, "repos", "apps", "app-a", "services", "svc-a")
		if err := afero.WriteFile(fs, filepath.Join(svcPath, "main.go"), []byte("svc"), 0o644); err != nil {
			t.Fatalf("setup svc: %v", err)
		}
		// Add a node_modules dir that should be ignored.
		if err := afero.WriteFile(fs, filepath.Join(svcPath, "node_modules", "some-pkg", "index.js"), []byte("pkg"), 0o644); err != nil {
			t.Fatalf("setup node_modules: %v", err)
		}
		// Seed .oceanignore to exclude node_modules.
		if err := afero.WriteFile(fs, filepath.Join(root, ".oceanignore"), []byte("node_modules/\n"), 0o644); err != nil {
			t.Fatalf("setup oceanignore: %v", err)
		}

		stagingPath, err := StageServiceBuildContext(fs, root, config, "app-a", "svc-a", io.Discard)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		ignoredPath := filepath.Join(stagingPath, "repos", "apps", "app-a", "services", "svc-a", "node_modules")
		if _, err := fs.Stat(ignoredPath); err == nil {
			t.Fatalf("expected node_modules to be excluded from staging")
		}
	})

	t.Run("domain__stage__oceaninclude_file_copied_to_staging_root", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/workspace"

		config := MakeConfig(nil, []WorkspaceApp{makeServiceInApp("app-a", "svc-a")}, nil)
		if err := writeTestWorkspaceConfig(fs, root, config); err != nil {
			t.Fatalf("setup config: %v", err)
		}

		svcPath := filepath.Join(root, "repos", "apps", "app-a", "services", "svc-a")
		if err := afero.WriteFile(fs, filepath.Join(svcPath, "main.go"), []byte("svc"), 0o644); err != nil {
			t.Fatalf("setup svc: %v", err)
		}

		// Create .oceaninclude listing pnpm-workspace.yaml.
		if err := afero.WriteFile(fs, filepath.Join(root, ".oceaninclude"), []byte("pnpm-workspace.yaml\n"), 0o644); err != nil {
			t.Fatalf("setup oceaninclude: %v", err)
		}
		// Create the workspace file to be included.
		if err := afero.WriteFile(fs, filepath.Join(root, "pnpm-workspace.yaml"), []byte("packages:\n  - repos/**\n"), 0o644); err != nil {
			t.Fatalf("setup pnpm-workspace: %v", err)
		}

		stagingPath, err := StageServiceBuildContext(fs, root, config, "app-a", "svc-a", io.Discard)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		includedFile := filepath.Join(stagingPath, "pnpm-workspace.yaml")
		if _, err := fs.Stat(includedFile); err != nil {
			t.Fatalf("expected pnpm-workspace.yaml at staging root: %v", err)
		}
	})
}

// writeTestWorkspaceConfig writes a WorkspaceConfig to ocean.workspace.json for tests that
// call StageServiceBuildContext (which reads the workspace root).
func writeTestWorkspaceConfig(fs afero.Fs, root string, config WorkspaceConfig) error {
	if err := fs.MkdirAll(root, 0o755); err != nil {
		return err
	}
	return WriteWorkspaceConfig(fs, config)
}
