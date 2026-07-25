package functions

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
	"github.com/spf13/afero"
)

func WireNewRepoVcs(fs afero.Fs, stdout io.Writer, stderr io.Writer, kind string, name string, app string) error {
	root, err := EnsureWorkspaceRoot(fs)
	if err != nil {
		return err
	}
	return WireNewRepoVcsAt(fs, root, stdout, stderr, kind, name, app)
}

func WireNewRepoVcsAt(fs afero.Fs, root string, stdout io.Writer, stderr io.Writer, kind string, name string, app string) error {
	return wireRepoVcsAt(fs, root, stdout, stderr, kind, name, app, true)
}

func wireRepoVcsAt(fs afero.Fs, root string, stdout io.Writer, stderr io.Writer, kind string, name string, app string, createRemote bool) error {
	target, err := vcsTaskTarget(kind, name, app)
	if err != nil {
		return err
	}
	if err := runVcsTask(fs, root, stdout, stderr, tokens.WorkspaceTaskInit, target.name, target.app); err != nil {
		return fmt.Errorf("init: %w", err)
	}
	if err := runVcsTask(fs, root, stdout, stderr, tokens.WorkspaceTaskInitialCommit, target.name, target.app); err != nil {
		return fmt.Errorf("initial_commit: %w", err)
	}
	if createRemote {
		if err := runVcsTask(fs, root, stdout, stderr, tokens.WorkspaceTaskCreateRemote, target.name, target.app); err != nil {
			fmt.Fprintf(stderr, "warning: create_remote failed; %s/%s is initialized locally but no remote was created: %v\n", kind, name, err)
		}
	}
	return nil
}

func InitRepoVcs(fs afero.Fs, stdout io.Writer, stderr io.Writer, kind string, name string, app string, remoteOverride string, noRemote bool) error {
	root, err := EnsureWorkspaceRoot(fs)
	if err != nil {
		return err
	}
	return InitRepoVcsAt(fs, root, stdout, stderr, kind, name, app, remoteOverride, noRemote)
}

func InitRepoVcsAt(fs afero.Fs, root string, stdout io.Writer, stderr io.Writer, kind string, name string, app string, remoteOverride string, noRemote bool) error {
	if err := ValidateRepoKindFlags(kind, app); err != nil {
		return err
	}

	config, err := ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	if !IsRegisteredInWorkspace(config, kind, name, app) {
		return fmt.Errorf("repo is not registered in workspace: %s/%s (run register or add first)", kind, name)
	}

	relPath, err := ResolveRepoPath(kind, name, app)
	if err != nil {
		return err
	}
	info, err := fs.Stat(relPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no directory at %s", relPath)
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s exists and is not a directory", relPath)
	}
	if _, err := fs.Stat(filepath.Join(relPath, ".git")); err == nil {
		return fmt.Errorf("%s is already a git repository", relPath)
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := wireRepoVcsAt(fs, root, stdout, stderr, kind, name, app, !noRemote); err != nil {
		return err
	}

	recordedRemote := resolveRecordedRemote(config, name, remoteOverride, noRemote, stderr)
	if err := UpdateConfig(fs, func(c WorkspaceConfig) (WorkspaceConfig, error) {
		setRemoteOnRepo(&c, kind, name, app, recordedRemote)
		return c, nil
	}); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "initialized git repo for %s/%s at %s (remote: %s)\n", kind, name, relPath, recordedRemote)
	return nil
}

func resolveRecordedRemote(config WorkspaceConfig, name string, remoteOverride string, noRemote bool, stderr io.Writer) string {
	if noRemote {
		return tokens.RemoteNone
	}
	if remoteOverride != "" {
		return remoteOverride
	}
	org := LoadWorkspaceVariables(config)["org"]
	if org == "" {
		fmt.Fprintf(stderr, "warning: workspace variable 'org' is not set; recording remote as %q (set variables.org or pass --remote)\n", tokens.RemoteNone)
		return tokens.RemoteNone
	}
	return fmt.Sprintf("%s/%s", org, name)
}

type vcsTarget struct {
	name string
	app  string
}

func vcsTaskTarget(kind string, name string, app string) (vcsTarget, error) {
	switch kind {
	case tokens.RepoKindLibrary:
		if app != "" {

			return vcsTarget{}, fmt.Errorf("app-scoped libraries inherit their app's git history; VCS wiring is not applicable")
		}
		return vcsTarget{name: name, app: ""}, nil
	case tokens.RepoKindApp,
		tokens.RepoKindProject,
		tokens.RepoKindTemplate,
		tokens.RepoKindInfra,
		tokens.RepoKindDocs:
		return vcsTarget{name: name, app: ""}, nil
	case tokens.RepoKindService:
		return vcsTarget{}, fmt.Errorf("services inherit their app's git history; VCS wiring is not applicable")
	}
	return vcsTarget{}, fmt.Errorf("unsupported kind for VCS wiring: %s", kind)
}

func runVcsTask(fs afero.Fs, root string, stdout io.Writer, stderr io.Writer, taskName string, target string, app string) error {
	config, err := ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	command, ok := config.Tasks[taskName]
	if !ok || command == "" {
		fmt.Fprintf(stdout, "workspace task %s skipped: no command configured\n", taskName)
		return nil
	}
	return RunWorkspaceTaskAt(fs, root, stdout, stderr, taskName, target, app)
}
