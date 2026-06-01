package functions

import (
	"github.com/spf13/afero"
	"path/filepath"
	"testing"
	"time"
)

func TestCalcDirHash(t *testing.T) {
	t.Run("domain__stable_inputs__returns_same_hash", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/"
		fixedTime := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
		filePath := filepath.Join(root, "alpha.txt")

		if err := afero.WriteFile(fs, filePath, []byte("alpha"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
		if err := fs.Chtimes(filePath, fixedTime, fixedTime); err != nil {
			t.Fatalf("chtimes: %v", err)
		}

		first, err := CalcDirHash(fs, root, nil)
		if err != nil {
			t.Fatalf("CalcDirHash: %v", err)
		}
		second, err := CalcDirHash(fs, root, nil)
		if err != nil {
			t.Fatalf("CalcDirHash: %v", err)
		}
		if first != second {
			t.Fatalf("expected stable hash")
		}
	})

	t.Run("boundary__empty_directory__returns_hash", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/"

		value, err := CalcDirHash(fs, root, nil)
		if err != nil {
			t.Fatalf("CalcDirHash: %v", err)
		}
		if value == "" {
			t.Fatalf("expected non-empty hash")
		}
	})

	t.Run("complement__missing_root__returns_error", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		missing := "/missing"

		if _, err := CalcDirHash(fs, missing, nil); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("chaos__ignored_directories__do_not_affect_hash", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/"
		fixedTime := time.Date(2024, 2, 3, 4, 5, 6, 0, time.UTC)
		keepPath := filepath.Join(root, "keep.txt")
		ignoredDir := filepath.Join(root, "node_modules")
		ignoredPath := filepath.Join(ignoredDir, "ignored.txt")

		if err := fs.MkdirAll(ignoredDir, 0o755); err != nil {
			t.Fatalf("mkdir ignored dir: %v", err)
		}
		if err := afero.WriteFile(fs, keepPath, []byte("keep"), 0o644); err != nil {
			t.Fatalf("write keep file: %v", err)
		}
		if err := afero.WriteFile(fs, ignoredPath, []byte("ignored"), 0o644); err != nil {
			t.Fatalf("write ignored file: %v", err)
		}
		if err := fs.Chtimes(keepPath, fixedTime, fixedTime); err != nil {
			t.Fatalf("chtimes keep: %v", err)
		}
		if err := fs.Chtimes(ignoredPath, fixedTime, fixedTime); err != nil {
			t.Fatalf("chtimes ignored: %v", err)
		}

		first, err := CalcDirHash(fs, root, []string{"node_modules/"})
		if err != nil {
			t.Fatalf("CalcDirHash: %v", err)
		}

		updatedTime := fixedTime.Add(2 * time.Hour)
		if err := afero.WriteFile(fs, ignoredPath, []byte("ignored update"), 0o644); err != nil {
			t.Fatalf("write ignored file: %v", err)
		}
		if err := fs.Chtimes(ignoredPath, updatedTime, updatedTime); err != nil {
			t.Fatalf("chtimes ignored: %v", err)
		}

		second, err := CalcDirHash(fs, root, []string{"node_modules/"})
		if err != nil {
			t.Fatalf("CalcDirHash: %v", err)
		}
		if first != second {
			t.Fatalf("expected hash to ignore node_modules changes")
		}
	})
}

// --- CollectRepoIgnorePatterns ---

func TestCollectRepoIgnorePatterns(t *testing.T) {
	t.Run("domain__collect__merges_workspace_and_repo_gitignores", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		workspaceRoot := "/workspace"
		repoPath := filepath.Join(workspaceRoot, "repos", "projects", "proj-a")

		if err := afero.WriteFile(fs, filepath.Join(workspaceRoot, ".gitignore"), []byte("node_modules\n.ocean\n"), 0o644); err != nil {
			t.Fatalf("write workspace gitignore: %v", err)
		}
		if err := afero.WriteFile(fs, filepath.Join(repoPath, ".gitignore"), []byte("/dist\ncoverage\n"), 0o644); err != nil {
			t.Fatalf("write repo gitignore: %v", err)
		}

		patterns, err := CollectRepoIgnorePatterns(fs, workspaceRoot, repoPath)
		if err != nil {
			t.Fatalf("CollectRepoIgnorePatterns: %v", err)
		}
		want := []string{"node_modules", ".ocean", "/dist", "coverage"}
		if len(patterns) != len(want) {
			t.Fatalf("expected %d patterns, got %d: %v", len(want), len(patterns), patterns)
		}
		for i, w := range want {
			if patterns[i] != w {
				t.Fatalf("pattern %d: got %q want %q", i, patterns[i], w)
			}
		}
	})

	t.Run("boundary__collect__repo_gitignore_absent_returns_workspace_patterns", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		workspaceRoot := "/workspace"
		repoPath := filepath.Join(workspaceRoot, "repos", "projects", "proj-a")

		if err := afero.WriteFile(fs, filepath.Join(workspaceRoot, ".gitignore"), []byte("node_modules\n"), 0o644); err != nil {
			t.Fatalf("write workspace gitignore: %v", err)
		}
		if err := fs.MkdirAll(repoPath, 0o755); err != nil {
			t.Fatalf("mkdir repo: %v", err)
		}

		patterns, err := CollectRepoIgnorePatterns(fs, workspaceRoot, repoPath)
		if err != nil {
			t.Fatalf("CollectRepoIgnorePatterns: %v", err)
		}
		if len(patterns) != 1 || patterns[0] != "node_modules" {
			t.Fatalf("expected only workspace patterns, got %v", patterns)
		}
	})

	t.Run("boundary__collect__repo_path_equal_to_workspace_does_not_double_read", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		workspaceRoot := "/workspace"

		if err := afero.WriteFile(fs, filepath.Join(workspaceRoot, ".gitignore"), []byte("node_modules\n"), 0o644); err != nil {
			t.Fatalf("write workspace gitignore: %v", err)
		}

		patterns, err := CollectRepoIgnorePatterns(fs, workspaceRoot, workspaceRoot)
		if err != nil {
			t.Fatalf("CollectRepoIgnorePatterns: %v", err)
		}
		if len(patterns) != 1 {
			t.Fatalf("expected 1 pattern (not duplicated), got %v", patterns)
		}
	})
}

// --- CalcRepoHash with repo-local .gitignore ---

func TestCalcRepoHash_RepoLocalGitignoreExcludesFiles(t *testing.T) {
	fs := afero.NewMemMapFs()
	workspaceRoot := "/workspace"
	repoPath := filepath.Join(workspaceRoot, "repos", "projects", "proj-a")
	fixedTime := time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC)

	// Workspace gitignore covers node_modules; repo gitignore covers /dist.
	if err := afero.WriteFile(fs, filepath.Join(workspaceRoot, ".gitignore"), []byte("node_modules\n"), 0o644); err != nil {
		t.Fatalf("write workspace gitignore: %v", err)
	}
	if err := afero.WriteFile(fs, filepath.Join(repoPath, ".gitignore"), []byte("/dist\n"), 0o644); err != nil {
		t.Fatalf("write repo gitignore: %v", err)
	}
	// Source file (in the hash).
	srcPath := filepath.Join(repoPath, "main.ts")
	if err := afero.WriteFile(fs, srcPath, []byte("export{}"), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	// Build artifact that must be excluded.
	distPath := filepath.Join(repoPath, "dist", "main.js")
	if err := afero.WriteFile(fs, distPath, []byte("v1"), 0o644); err != nil {
		t.Fatalf("write dist: %v", err)
	}
	for _, p := range []string{filepath.Join(repoPath, ".gitignore"), srcPath, distPath} {
		if err := fs.Chtimes(p, fixedTime, fixedTime); err != nil {
			t.Fatalf("chtimes %s: %v", p, err)
		}
	}

	first, err := CalcRepoHash(fs, workspaceRoot, repoPath)
	if err != nil {
		t.Fatalf("CalcRepoHash first: %v", err)
	}

	// Mutate dist/ only — if the repo-local gitignore is respected, the hash stays the same.
	updated := fixedTime.Add(time.Hour)
	if err := afero.WriteFile(fs, distPath, []byte("v2"), 0o644); err != nil {
		t.Fatalf("rewrite dist: %v", err)
	}
	if err := fs.Chtimes(distPath, updated, updated); err != nil {
		t.Fatalf("chtimes dist: %v", err)
	}

	second, err := CalcRepoHash(fs, workspaceRoot, repoPath)
	if err != nil {
		t.Fatalf("CalcRepoHash second: %v", err)
	}
	if first != second {
		t.Fatalf("expected hash to be stable across dist/ changes when repo-local .gitignore excludes /dist; first=%s second=%s", first, second)
	}
}
