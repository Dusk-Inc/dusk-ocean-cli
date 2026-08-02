package functions

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/models"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

func StopService(cmd *cobra.Command, fs afero.Fs, appName string, serviceName string, selection models.GroupSelection) error {
	root, err := EnsureWorkspaceRoot(fs)
	if err != nil {
		return err
	}

	config, err := ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}

	resolvedApp, resolvedSvc, err := ResolveContainTarget(config, appName, serviceName)
	if err != nil {
		return err
	}

	servicePath := filepath.Join(root, "repos", "apps", resolvedApp, "services", resolvedSvc)
	resolved, err := resolveRepoTask(fs, root, config, servicePath, tokens.RepoKindService, resolvedApp, resolvedSvc, "stop", selection)
	stopTask := resolved.Command
	if err != nil {
		return err
	}
	if strings.TrimSpace(stopTask) == "" {
		fmt.Fprintf(cmd.OutOrStdout(), "stop skipped for service %s/%s: no stop task\n", resolvedApp, resolvedSvc)
		return nil
	}

	envFile, err := LoadEnvFile(fs, root, cmd.OutOrStdout())
	if err != nil {
		return err
	}

	execCmd := exec.Command("bash", "-lc", stopTask)
	execCmd.Dir = servicePath
	execCmd.Env = mergeEnvForExec(envFile)
	execCmd.Stdout = cmd.OutOrStdout()
	execCmd.Stderr = cmd.ErrOrStderr()
	execCmd.Stdin = cmd.InOrStdin()
	return execCmd.Run()
}

func StopApp(cmd *cobra.Command, fs afero.Fs, appName string, selection models.GroupSelection) error {
	root, err := EnsureWorkspaceRoot(fs)
	if err != nil {
		return err
	}

	config, err := ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}

	if FindAppIndex(config, appName) == -1 {
		return fmt.Errorf("app not found: %s", appName)
	}

	appPath := filepath.Join(root, "repos", "apps", appName)
	resolved, err := resolveRepoTask(fs, root, config, appPath, tokens.RepoKindApp, appName, appName, "stop", selection)
	stopTask := resolved.Command
	if err != nil {
		return err
	}
	if strings.TrimSpace(stopTask) == "" {
		fmt.Fprintf(cmd.OutOrStdout(), "stop skipped for app %s: no stop task\n", appName)
		return nil
	}

	envFile, err := LoadEnvFile(fs, root, cmd.OutOrStdout())
	if err != nil {
		return err
	}

	execCmd := exec.Command("bash", "-lc", stopTask)
	execCmd.Dir = appPath
	execCmd.Env = mergeEnvForExec(envFile)
	execCmd.Stdout = cmd.OutOrStdout()
	execCmd.Stderr = cmd.ErrOrStderr()
	execCmd.Stdin = cmd.InOrStdin()
	return execCmd.Run()
}
