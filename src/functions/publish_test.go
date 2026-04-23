package functions

import (
	"path/filepath"
	"strings"
	"testing"
)

// --- MakePublishHashPath ---

func TestMakePublishHashPath(t *testing.T) {
	t.Run("domain__make_publish_hash_path__replaces_build_segment", func(t *testing.T) {
		build := filepath.Join("root", ".ocean", "hashes", "build", "projects", "proj-a.hash")
		got := MakePublishHashPath(build)
		if !strings.Contains(got, filepath.Join(".ocean", "hashes", "publish", "projects", "proj-a.hash")) {
			t.Fatalf("expected publish segment in path, got %q", got)
		}
		if strings.Contains(got, filepath.Join("hashes", "build")) {
			t.Fatalf("build segment not replaced: %q", got)
		}
	})

	t.Run("boundary__make_publish_hash_path__no_change_when_no_build_segment", func(t *testing.T) {
		src := filepath.Join("root", ".ocean", "hashes", "check", "projects", "proj-a.hash")
		got := MakePublishHashPath(src)
		if got != filepath.Clean(src) {
			t.Fatalf("expected input preserved, got %q", got)
		}
	})
}
