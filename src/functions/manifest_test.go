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
					Kind:     "global-lib",
					Name:     "my-lib",
					Hash:     "abc123def456",
					Dirty:    false,
					BuildRun: true,
					CheckRun: false,
					HashedAt: "2026-01-01T00:00:00Z",
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
		if entry.Hash != "abc123def456" {
			t.Fatalf("hash mismatch: got %s", entry.Hash)
		}
		if entry.Name != "my-lib" {
			t.Fatalf("name mismatch: got %s", entry.Name)
		}
		if !entry.BuildRun {
			t.Fatalf("expected build_run=true")
		}
		if entry.CheckRun {
			t.Fatalf("expected check_run=false")
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

	t.Run("domain__hash_all_repos__new_entry_has_dirty_true_and_runs_false", func(t *testing.T) {
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
		if !entry.Dirty {
			t.Fatalf("expected dirty=true for new entry")
		}
		if entry.BuildRun {
			t.Fatalf("expected build_run=false for new entry")
		}
		if entry.CheckRun {
			t.Fatalf("expected check_run=false for new entry")
		}
	})

	t.Run("domain__hash_all_repos__unchanged_hash_preserves_build_and_check_run", func(t *testing.T) {
		fs, root, config := setupManifestWorkspace(t)
		cmd := makeTestCmd(&bytes.Buffer{})

		// First hash pass — creates new dirty entries.
		if err := HashAllRepos(cmd, fs, root, config); err != nil {
			t.Fatalf("first hash: %v", err)
		}
		// Simulate a successful build for the lib.
		if err := SetManifestBuildRun(fs, root, "lib:global:lib-a"); err != nil {
			t.Fatalf("set build_run: %v", err)
		}

		// Second hash pass — files unchanged, hash must match.
		if err := HashAllRepos(cmd, fs, root, config); err != nil {
			t.Fatalf("second hash: %v", err)
		}
		m, err := ReadManifest(fs, root)
		if err != nil {
			t.Fatalf("read manifest: %v", err)
		}
		entry := m.Repos["lib:global:lib-a"]
		if entry.Dirty {
			t.Fatalf("expected dirty=false when hash unchanged")
		}
		if !entry.BuildRun {
			t.Fatalf("expected build_run=true to be preserved when hash unchanged")
		}
	})

	t.Run("domain__hash_all_repos__changed_hash_resets_runs_and_marks_dirty", func(t *testing.T) {
		fs, root, config := setupManifestWorkspace(t)
		cmd := makeTestCmd(&bytes.Buffer{})

		// First hash pass.
		if err := HashAllRepos(cmd, fs, root, config); err != nil {
			t.Fatalf("first hash: %v", err)
		}
		// Simulate a completed build.
		if err := SetManifestBuildRun(fs, root, "lib:global:lib-a"); err != nil {
			t.Fatalf("set build_run: %v", err)
		}

		// Mutate the lib source to force a different hash.
		libPath := filepath.Join(root, "repos", "libs", "lib-a")
		if err := afero.WriteFile(fs, filepath.Join(libPath, "lib.go"), []byte("lib-changed"), 0o644); err != nil {
			t.Fatalf("modify source: %v", err)
		}

		// Second hash pass — lib-a hash must change.
		if err := HashAllRepos(cmd, fs, root, config); err != nil {
			t.Fatalf("second hash: %v", err)
		}
		m, err := ReadManifest(fs, root)
		if err != nil {
			t.Fatalf("read manifest: %v", err)
		}
		entry := m.Repos["lib:global:lib-a"]
		if !entry.Dirty {
			t.Fatalf("expected dirty=true after source change")
		}
		if entry.BuildRun {
			t.Fatalf("expected build_run=false after hash change")
		}
		if entry.CheckRun {
			t.Fatalf("expected check_run=false after hash change")
		}
	})
}

// --- HashSingleRepo ---

func TestHashSingleRepo(t *testing.T) {
	t.Run("domain__hash_single_repo__updates_only_target_entry", func(t *testing.T) {
		fs, root, config := setupManifestWorkspace(t)
		cmd := makeTestCmd(&bytes.Buffer{})

		// Pre-populate both entries so we can verify only one changes.
		if err := HashAllRepos(cmd, fs, root, config); err != nil {
			t.Fatalf("initial hash: %v", err)
		}

		// Mutate only lib-a source.
		libPath := filepath.Join(root, "repos", "libs", "lib-a")
		if err := afero.WriteFile(fs, filepath.Join(libPath, "lib.go"), []byte("lib-v2"), 0o644); err != nil {
			t.Fatalf("modify source: %v", err)
		}

		m0, _ := ReadManifest(fs, root)
		svcHashBefore := m0.Repos["service:app-a:svc-a"].Hash

		// Hash only lib-a.
		if err := HashSingleRepo(cmd, fs, root, config, "lib-a"); err != nil {
			t.Fatalf("single hash: %v", err)
		}

		m1, err := ReadManifest(fs, root)
		if err != nil {
			t.Fatalf("read manifest: %v", err)
		}
		// lib-a entry must be updated (dirty, new hash).
		libEntry := m1.Repos["lib:global:lib-a"]
		if !libEntry.Dirty {
			t.Fatalf("expected lib-a to be dirty after source change")
		}
		// svc-a entry must be unchanged.
		if m1.Repos["service:app-a:svc-a"].Hash != svcHashBefore {
			t.Fatalf("svc-a hash should not change when only lib-a was targeted")
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

// --- SetManifestBuildRun ---

func TestSetManifestBuildRun(t *testing.T) {
	t.Run("domain__set_manifest_build_run__sets_build_run_true", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/root"
		m := Manifest{
			Repos: map[string]ManifestEntry{
				"lib:global:lib-a": {Kind: "global-lib", Name: "lib-a", BuildRun: false},
			},
		}
		if err := WriteManifest(fs, root, m); err != nil {
			t.Fatalf("setup: %v", err)
		}

		if err := SetManifestBuildRun(fs, root, "lib:global:lib-a"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, _ := ReadManifest(fs, root)
		if !got.Repos["lib:global:lib-a"].BuildRun {
			t.Fatalf("expected build_run=true")
		}
	})

	t.Run("complement__set_manifest_build_run__no_op_when_entry_absent", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/root"
		// Write a manifest with no entries.
		if err := WriteManifest(fs, root, Manifest{Repos: map[string]ManifestEntry{}}); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := SetManifestBuildRun(fs, root, "lib:global:ghost"); err != nil {
			t.Fatalf("expected no-op, got error: %v", err)
		}
	})
}

// --- SetManifestCheckRun ---

func TestSetManifestCheckRun(t *testing.T) {
	t.Run("domain__set_manifest_check_run__sets_check_run_true", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/root"
		m := Manifest{
			Repos: map[string]ManifestEntry{
				"service:app-a:svc-a": {Kind: "service", App: "app-a", Name: "svc-a", CheckRun: false},
			},
		}
		if err := WriteManifest(fs, root, m); err != nil {
			t.Fatalf("setup: %v", err)
		}

		if err := SetManifestCheckRun(fs, root, "service:app-a:svc-a"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, _ := ReadManifest(fs, root)
		if !got.Repos["service:app-a:svc-a"].CheckRun {
			t.Fatalf("expected check_run=true")
		}
	})

	t.Run("complement__set_manifest_check_run__no_op_when_entry_absent", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/root"
		if err := WriteManifest(fs, root, Manifest{Repos: map[string]ManifestEntry{}}); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := SetManifestCheckRun(fs, root, "service:app-a:ghost"); err != nil {
			t.Fatalf("expected no-op, got error: %v", err)
		}
	})
}
