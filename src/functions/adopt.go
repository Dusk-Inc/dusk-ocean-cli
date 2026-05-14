package functions

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
	"github.com/spf13/afero"
)

var gitClone = func(url, dest string, stdout, stderr io.Writer) error {
	cmd := exec.Command("git", "clone", url, dest)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

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

	workspaceConfig, err := ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	if IsRegisteredInWorkspace(workspaceConfig, kind, name, app) {
		return fmt.Errorf("repo is already registered in workspace: %s/%s", kind, name)
	}

	if info, err := fs.Stat(relPath); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("%s exists and is not a directory", relPath)
		}
		return fmt.Errorf("directory already exists at %s but is not managed; run register instead", relPath)
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := fs.MkdirAll(filepath.Dir(relPath), 0o755); err != nil {
		return err
	}

	if err := gitClone(remoteURL, relPath, stdout, stderr); err != nil {
		return fmt.Errorf("git clone %s: %w", remoteURL, err)
	}

	configPath := filepath.Join(relPath, "ocean.config.json")
	if _, err := fs.Stat(configPath); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		starterType := kind
		if kind == tokens.RepoKindTemplate {

			starterType = templateKind
		}
		if err := WriteStarterRepoConfig(fs, relPath, name, starterType); err != nil {
			return err
		}
	}

	if err := registerEntryInWorkspace(fs, kind, name, app, remoteURL, templateKind); err != nil {
		return err
	}

	if kind == tokens.RepoKindApp {
		if err := RegisterDiscoveredAppSubRepos(fs, stdout, name); err != nil {
			return err
		}
	}

	fmt.Fprintf(stdout, "adopted %s/%s at %s\n", kind, name, relPath)
	return nil
}

func deriveNameFromRemote(remoteURL string) string {
	trimmed := strings.TrimSuffix(remoteURL, "/")
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		return ""
	}

	trimmed = strings.TrimSuffix(trimmed, ".git")

	for i := len(trimmed) - 1; i >= 0; i-- {
		if trimmed[i] == '/' || trimmed[i] == ':' {
			return trimmed[i+1:]
		}
	}
	return trimmed
}
