package cmd

import (
	"strings"
	"testing"
)

// The usage-error process edge: `refresh --no-deps` without `--repo` must fail before
// touching the workspace, returning a non-nil error (which cobra surfaces as a non-zero
// exit and a stderr diagnostic). This guard runs ahead of any filesystem/config access,
// so it is exercisable without a real workspace by driving the command's RunE directly.
func TestRefreshCommandUsage(t *testing.T) {
	t.Run("error__refreshCommand__noDepsWithoutRepoIsUsageError", func(t *testing.T) {
		if err := refreshCmd.Flags().Set("no-deps", "true"); err != nil {
			t.Fatalf("set --no-deps: %v", err)
		}
		t.Cleanup(func() {
			_ = refreshCmd.Flags().Set("no-deps", "false")
			_ = refreshCmd.Flags().Set("repo", "")
		})

		err := refreshCmd.RunE(refreshCmd, []string{})
		if err == nil {
			t.Fatalf("expected a usage error for --no-deps without --repo")
		}
		if !strings.Contains(err.Error(), "--no-deps requires --repo") {
			t.Fatalf("error should explain the usage violation, got: %v", err)
		}
	})
}
