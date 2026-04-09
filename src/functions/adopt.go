package functions

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
)

// gitClone is the package-level injection point used by AdoptRepo to run
// `git clone <url> <dest>`. Tests replace this with a stub so they can
// exercise adopt's flow without hitting a real network or git binary.
var gitClone = func(url, dest string, stdout, stderr io.Writer) error {
	cmd := exec.Command("git", "clone", url, dest)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// AdoptRepo clones an external repo into the deterministic workspace path
// for the given kind, writes a starter ocean.config.json, and registers a
// new entry in ocean.workspace.json. The remote URL is stored on the new
// entry as the {{repo:remote}} value.
//
// Behavior matrix for the target directory:
//
//	does not exist                  → clone, write starter, register
//	exists, no ocean.config.json    → error suggesting register
//	exists, has ocean.config.json   → already-registered error
//
// On any failure after the clone, the cloned directory is left in place
// so the developer can investigate. AdoptRepo intentionally does not
// half-rollback on the user's filesystem.
func AdoptRepo(fs afero.Fs, stdout io.Writer, stderr io.Writer, remoteURL string, kind string, name string, app string, templateKind string) error {
	if strings.TrimSpace(remoteURL) == "" {
		return fmt.Errorf("remote URL is required")
	}
	if name == "" {
		name = deriveNameFromRemote(remoteURL)
		if name == "" {
			return fmt.Errorf("--name is required when it cannot be derived from the remote URL")
		}
	}
	if err := ValidateRepoKindFlags(kind, app); err != nil {
		return err
	}
	if err := ValidateTemplateKindFlag(kind, templateKind); err != nil {
		return err
	}

	relPath, err := ResolveRepoPath(kind, name, app)
	if err != nil {
		return err
	}

	if info, err := fs.Stat(relPath); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("%s exists and is not a directory", relPath)
		}
		configPath := filepath.Join(relPath, "ocean.config.json")
		if _, err := fs.Stat(configPath); err == nil {
			return fmt.Errorf("repo is already registered at %s", relPath)
		} else if !os.IsNotExist(err) {
			return err
		}
		return fmt.Errorf("directory already exists at %s but is not managed; run register instead", relPath)
	} else if !os.IsNotExist(err) {
		return err
	}

	// Ensure the parent directory exists so the clone has a place to land.
	if err := fs.MkdirAll(filepath.Dir(relPath), 0o755); err != nil {
		return err
	}

	if err := gitClone(remoteURL, relPath, stdout, stderr); err != nil {
		return fmt.Errorf("git clone %s: %w", remoteURL, err)
	}

	starterType := kind
	if kind == "template" {
		// A template repo's ocean.config.json declares the kind it scaffolds,
		// so the runtime can route it correctly via ListTemplatesByType.
		starterType = templateKind
	}
	if err := WriteStarterRepoConfig(fs, relPath, name, starterType); err != nil {
		return err
	}
	if err := registerEntryInWorkspace(fs, kind, name, app, remoteURL, templateKind); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "adopted %s/%s at %s\n", kind, name, relPath)
	return nil
}

// deriveNameFromRemote returns a sensible default name from a git URL,
// e.g. "git@github.com:dusk-inc/svc-a.git" → "svc-a". Returns "" if it
// cannot find a sensible candidate.
func deriveNameFromRemote(remoteURL string) string {
	trimmed := strings.TrimSuffix(remoteURL, "/")
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		return ""
	}
	// Strip trailing .git
	trimmed = strings.TrimSuffix(trimmed, ".git")
	// Take everything after the last "/" or ":" (handles ssh form).
	for i := len(trimmed) - 1; i >= 0; i-- {
		if trimmed[i] == '/' || trimmed[i] == ':' {
			return trimmed[i+1:]
		}
	}
	return trimmed
}
