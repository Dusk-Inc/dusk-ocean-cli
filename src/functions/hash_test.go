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
