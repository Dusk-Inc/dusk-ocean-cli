package functions

import (
	"fmt"
	"io"
	"os/exec"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
	"github.com/spf13/afero"
)

var runShell = func(workdir string, command string, stdout io.Writer, stderr io.Writer) error {
	cmd := exec.Command("bash", "-lc", command)
	cmd.Dir = workdir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// RunWorkspaceTask runs a named workspace task against one target repo, discovering the
// workspace root first. It is the entry point for a caller that has no root in hand;
// RunWorkspaceTaskAt is the same work with the root already known.
func RunWorkspaceTask(fs afero.Fs, stdout io.Writer, stderr io.Writer, taskName string, target string, app string) error {
	root, err := EnsureWorkspaceRoot(fs)
	if err != nil {
		return err
	}
	return RunWorkspaceTaskAt(fs, root, stdout, stderr, taskName, target, app)
}

// RunWorkspaceTaskAt runs a named workspace task against one target repo under an already
// known root. It resolves the target's kind, expands the task template against the env,
// workspace, and repo namespaces, and runs the result as a shell command from the workspace
// root. An unresolved variable fails the task rather than running a partially expanded
// command.
func RunWorkspaceTaskAt(fs afero.Fs, root string, stdout io.Writer, stderr io.Writer, taskName string, target string, app string) error {
	if taskName == "" {
		return fmt.Errorf("--name is required")
	}
	if target == "" {
		return fmt.Errorf("--target is required")
	}

	config, err := ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}

	template, ok := config.Tasks[taskName]
	if !ok {
		return fmt.Errorf("workspace task not found: %s", taskName)
	}

	kind, err := ResolveRepoKindByName(config, app, target)
	if err != nil {
		return err
	}

	envValues, err := LoadEnvFile(fs, root, stdout)
	if err != nil {
		return err
	}
	repoVars, err := BuildRepoVariables(config, kind, app, target)
	if err != nil {
		return err
	}

	ctx := VariableContext{
		Env:   envValues,
		Var:   LoadWorkspaceVariables(config),
		Ocean: map[string]string{},
		Repo:  repoVars,
	}

	expanded, err := Substitute(template, ctx)
	if err != nil {
		return fmt.Errorf("task %s: %w", taskName, err)
	}

	fmt.Fprintf(stdout, "running workspace task %s against %s/%s\n", taskName, kind, target)
	return runShell(root, expanded, stdout, stderr)
}

// ResolveRepoKindByName returns the registered kind of the repo called name, searching within
// app when one is given and across the workspace otherwise. A name matching more than one kind
// is an error rather than a first-match win, since guessing would silently run the task
// against the wrong repo.
func ResolveRepoKindByName(config WorkspaceConfig, app string, name string) (string, error) {
	if app != "" {
		appIdx := FindAppIndex(config, app)
		if appIdx == -1 {
			return "", fmt.Errorf("app not registered in workspace: %s", app)
		}
		var matches []string
		if FindServiceIndex(config.Apps[appIdx], name) != -1 {
			matches = append(matches, tokens.RepoKindService)
		}
		if FindAppLibraryIndex(config.Apps[appIdx], name) != -1 {
			matches = append(matches, tokens.RepoKindLibrary)
		}
		switch len(matches) {
		case 0:
			return "", fmt.Errorf("repo %s not found in app %s", name, app)
		case 1:
			return matches[0], nil
		default:
			return "", fmt.Errorf("ambiguous target %s in app %s: matches %v", name, app, matches)
		}
	}

	var matches []string
	if FindProjectIndex(config, name) != -1 {
		matches = append(matches, tokens.RepoKindProject)
	}
	if FindGlobalLibraryIndex(config, name) != -1 {
		matches = append(matches, tokens.RepoKindLibrary)
	}
	if FindAppIndex(config, name) != -1 {
		matches = append(matches, tokens.RepoKindApp)
	}
	if FindTemplateIndex(config, name) != -1 {
		matches = append(matches, tokens.RepoKindTemplate)
	}
	if FindInfraIndex(config, name) != -1 {
		matches = append(matches, tokens.RepoKindInfra)
	}
	if FindDocsIndex(config, name) != -1 {
		matches = append(matches, tokens.RepoKindDocs)
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("repo %s not found at workspace top level (try --app for service/app-scoped library targets)", name)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("ambiguous target %s: matches %v; disambiguate with --app or rename", name, matches)
	}
}
