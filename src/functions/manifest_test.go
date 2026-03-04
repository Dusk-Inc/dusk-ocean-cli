package functions

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
)

// --- helpers ---

// setupManifestWorkspace creates a MemMapFs with a workspace config containing
// one global lib and one app with one service, seeded with source files.
func setupManifestWorkspace(t *testing.T) (afero.Fs, string, WorkspaceConfig) {
	t.Helper()
	fs := afero.NewMemMapFs()
	root := "/workspace"

	libA := WorkspaceLibrary{Name: "lib-a", Deps: []WorkspaceDep{}}
	svcA := WorkspaceService{Name: "svc-a", Deps: []WorkspaceDep{}}
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

	libPath := filepath.Join(root, "repos", "libs", "lib-a")
	svcPath := filepath.Join(root, "repos", "apps", "app-a", "services", "svc-a")
	if err := afero.WriteFile(fs, filepath.Join(libPath, "lib.go"), []byte("lib"), 0o644); err != nil {
		t.Fatalf("setup lib: %v", err)
	}
	if err := afero.WriteFile(fs, filepath.Join(svcPath, "main.go"), []byte("main"), 0o644); err != nil {
		t.Fatalf("setup svc: %v", err)
	}
	return fs, root, config
}

// --- ReadManifest ---

func TestReadManifest(t *testing.T) {
	t.Run("domain__read_manifest__returns_empty_when_file_absent", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		m, err := ReadManifest(fs, "/root")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m.Repos == nil {
			t.Fatalf("expected non-nil Repos map")
		}
		if len(m.Repos) != 0 {
			t.Fatalf("expected empty Repos, got %d entries", len(m.Repos))
		}
	})

	t.Run("domain__write_then_read__roundtrip_preserves_entries", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/root"
		original := Manifest{
			Repos: map[string]ManifestEntry{
				"lib:global:my-lib": {
					Kind:      "global-lib",
					Name:      "my-lib",
					BuildHash: "abc123",
					CheckHash: "def456",
				},
			},
		}
		if err := WriteManifest(fs, root, original); err != nil {
			t.Fatalf("write: %v", err)
		}
		got, err := ReadManifest(fs, root)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		entry, ok := got.Repos["lib:global:my-lib"]
		if !ok {
			t.Fatalf("entry not found after roundtrip")
		}
		if entry.BuildHash != "abc123" {
			t.Fatalf("build_hash mismatch: got %s", entry.BuildHash)
		}
		if entry.CheckHash != "def456" {
			t.Fatalf("check_hash mismatch: got %s", entry.CheckHash)
		}
		if entry.Name != "my-lib" {
			t.Fatalf("name mismatch: got %s", entry.Name)
		}
	})
}

// --- HashAllRepos ---

func TestHashAllRepos(t *testing.T) {
	t.Run("domain__hash_all_repos__creates_entries_for_all_registered_nodes", func(t *testing.T) {
		fs, root, config := setupManifestWorkspace(t)
		cmd := makeTestCmd(&bytes.Buffer{})

		if err := HashAllRepos(cmd, fs, root, config); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		m, err := ReadManifest(fs, root)
		if err != nil {
			t.Fatalf("read manifest: %v", err)
		}
		// Expect entries for lib-a (global-lib) and svc-a (service in app-a).
		if _, ok := m.Repos["lib:global:lib-a"]; !ok {
			t.Fatalf("expected entry for lib:global:lib-a")
		}
		if _, ok := m.Repos["service:app-a:svc-a"]; !ok {
			t.Fatalf("expected entry for service:app-a:svc-a")
		}
	})

	t.Run("domain__hash_all_repos__new_entry_has_empty_hashes", func(t *testing.T) {
		fs, root, config := setupManifestWorkspace(t)
		cmd := makeTestCmd(&bytes.Buffer{})

		if err := HashAllRepos(cmd, fs, root, config); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		m, err := ReadManifest(fs, root)
		if err != nil {
			t.Fatalf("read manifest: %v", err)
		}
		entry := m.Repos["lib:global:lib-a"]
		if entry.BuildHash != "" {
			t.Fatalf("expected empty build_hash for new entry, got %s", entry.BuildHash)
		}
		if entry.CheckHash != "" {
			t.Fatalf("expected empty check_hash for new entry, got %s", entry.CheckHash)
		}
		if entry.ContainHash != "" {
			t.Fatalf("expected empty contain_hash for new entry, got %s", entry.ContainHash)
		}
	})

	t.Run("domain__hash_all_repos__does_not_overwrite_existing_entries", func(t *testing.T) {
		fs, root, config := setupManifestWorkspace(t)
		cmd := makeTestCmd(&bytes.Buffer{})

		// First pass — creates entries.
		if err := HashAllRepos(cmd, fs, root, config); err != nil {
			t.Fatalf("first hash: %v", err)
		}
		// Simulate a successful build for the lib.
		if err := SetManifestBuildHash(fs, root, "lib:global:lib-a", "hash-after-build"); err != nil {
			t.Fatalf("set build_hash: %v", err)
		}

		// Second pass — entries already exist, must not overwrite.
		if err := HashAllRepos(cmd, fs, root, config); err != nil {
			t.Fatalf("second hash: %v", err)
		}
		m, err := ReadManifest(fs, root)
		if err != nil {
			t.Fatalf("read manifest: %v", err)
		}
		entry := m.Repos["lib:global:lib-a"]
		if entry.BuildHash != "hash-after-build" {
			t.Fatalf("expected build_hash to be preserved, got %s", entry.BuildHash)
		}
	})
}

// --- HashSingleRepo ---

func TestHashSingleRepo(t *testing.T) {
	t.Run("domain__hash_single_repo__creates_entry_for_target", func(t *testing.T) {
		fs, root, config := setupManifestWorkspace(t)
		cmd := makeTestCmd(&bytes.Buffer{})

		if err := HashSingleRepo(cmd, fs, root, config, "lib-a"); err != nil {
			t.Fatalf("single hash: %v", err)
		}

		m, err := ReadManifest(fs, root)
		if err != nil {
			t.Fatalf("read manifest: %v", err)
		}
		entry, ok := m.Repos["lib:global:lib-a"]
		if !ok {
			t.Fatalf("expected entry for lib:global:lib-a")
		}
		if entry.Kind != "global-lib" {
			t.Fatalf("expected kind=global-lib, got %s", entry.Kind)
		}
		if entry.Name != "lib-a" {
			t.Fatalf("expected name=lib-a, got %s", entry.Name)
		}
	})

	t.Run("domain__hash_single_repo__does_not_overwrite_existing_entry", func(t *testing.T) {
		fs, root, config := setupManifestWorkspace(t)
		cmd := makeTestCmd(&bytes.Buffer{})

		// Create entry and set a build hash.
		if err := HashSingleRepo(cmd, fs, root, config, "lib-a"); err != nil {
			t.Fatalf("first hash: %v", err)
		}
		if err := SetManifestBuildHash(fs, root, "lib:global:lib-a", "existing-hash"); err != nil {
			t.Fatalf("set build_hash: %v", err)
		}

		// Hash again — must not reset the build hash.
		if err := HashSingleRepo(cmd, fs, root, config, "lib-a"); err != nil {
			t.Fatalf("second hash: %v", err)
		}
		m, _ := ReadManifest(fs, root)
		if m.Repos["lib:global:lib-a"].BuildHash != "existing-hash" {
			t.Fatalf("expected build_hash preserved, got %s", m.Repos["lib:global:lib-a"].BuildHash)
		}
	})

	t.Run("complement__hash_single_repo__target_not_found_returns_error", func(t *testing.T) {
		fs, root, config := setupManifestWorkspace(t)
		cmd := makeTestCmd(&bytes.Buffer{})

		err := HashSingleRepo(cmd, fs, root, config, "ghost-repo")
		if err == nil {
			t.Fatalf("expected error for unknown target")
		}
	})
}

// --- SetManifestBuildHash ---

func TestSetManifestBuildHash(t *testing.T) {
	t.Run("domain__set_manifest_build_hash__stores_hash", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/root"
		m := Manifest{
			Repos: map[string]ManifestEntry{
				"lib:global:lib-a": {Kind: "global-lib", Name: "lib-a"},
			},
		}
		if err := WriteManifest(fs, root, m); err != nil {
			t.Fatalf("setup: %v", err)
		}

		if err := SetManifestBuildHash(fs, root, "lib:global:lib-a", "build-hash-123"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, _ := ReadManifest(fs, root)
		if got.Repos["lib:global:lib-a"].BuildHash != "build-hash-123" {
			t.Fatalf("expected build_hash=build-hash-123, got %s", got.Repos["lib:global:lib-a"].BuildHash)
		}
	})

	t.Run("complement__set_manifest_build_hash__no_op_when_entry_absent", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/root"
		if err := WriteManifest(fs, root, Manifest{Repos: map[string]ManifestEntry{}}); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := SetManifestBuildHash(fs, root, "lib:global:ghost", "some-hash"); err != nil {
			t.Fatalf("expected no-op, got error: %v", err)
		}
	})
}

// --- SetManifestCheckHash ---

func TestSetManifestCheckHash(t *testing.T) {
	t.Run("domain__set_manifest_check_hash__stores_hash", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/root"
		m := Manifest{
			Repos: map[string]ManifestEntry{
				"service:app-a:svc-a": {Kind: "service", App: "app-a", Name: "svc-a"},
			},
		}
		if err := WriteManifest(fs, root, m); err != nil {
			t.Fatalf("setup: %v", err)
		}

		if err := SetManifestCheckHash(fs, root, "service:app-a:svc-a", "check-hash-456"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, _ := ReadManifest(fs, root)
		if got.Repos["service:app-a:svc-a"].CheckHash != "check-hash-456" {
			t.Fatalf("expected check_hash=check-hash-456, got %s", got.Repos["service:app-a:svc-a"].CheckHash)
		}
	})

	t.Run("complement__set_manifest_check_hash__no_op_when_entry_absent", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/root"
		if err := WriteManifest(fs, root, Manifest{Repos: map[string]ManifestEntry{}}); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := SetManifestCheckHash(fs, root, "service:app-a:ghost", "some-hash"); err != nil {
			t.Fatalf("expected no-op, got error: %v", err)
		}
	})
}

// --- SetManifestContainHash ---

func TestSetManifestContainHash(t *testing.T) {
	t.Run("domain__set_manifest_contain_hash__stores_hash", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/root"
		m := Manifest{
			Repos: map[string]ManifestEntry{
				"service:app-a:svc-a": {Kind: "service", App: "app-a", Name: "svc-a"},
			},
		}
		if err := WriteManifest(fs, root, m); err != nil {
			t.Fatalf("setup: %v", err)
		}

		if err := SetManifestContainHash(fs, root, "service:app-a:svc-a", "contain-hash-789"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, _ := ReadManifest(fs, root)
		if got.Repos["service:app-a:svc-a"].ContainHash != "contain-hash-789" {
			t.Fatalf("expected contain_hash=contain-hash-789, got %s", got.Repos["service:app-a:svc-a"].ContainHash)
		}
	})

	t.Run("complement__set_manifest_contain_hash__no_op_when_entry_absent", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/root"
		if err := WriteManifest(fs, root, Manifest{Repos: map[string]ManifestEntry{}}); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := SetManifestContainHash(fs, root, "service:app-a:ghost", "some-hash"); err != nil {
			t.Fatalf("expected no-op, got error: %v", err)
		}
	})
}
