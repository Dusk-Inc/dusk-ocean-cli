package functions

import (
	"fmt"
	"io"

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
	target, err := vcsTaskTarget(kind, name, app)
	if err != nil {
		return err
	}
	if err := runVcsTask(fs, root, stdout, stderr, tokens.WorkspaceTaskCreateRemote, target.name, target.app); err != nil {
		return fmt.Errorf("create_remote: %w", err)
	}
	if err := runVcsTask(fs, root, stdout, stderr, tokens.WorkspaceTaskCheckoutNew, target.name, target.app); err != nil {
		return fmt.Errorf("checkout_new: %w", err)
	}
	return nil
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
