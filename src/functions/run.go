package functions

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

func PreflightService(cmd *cobra.Command, fs afero.Fs, root string, config WorkspaceConfig, appName string, serviceName string) error {
	serviceNode, err := MakeServiceNode(config, appName, serviceName)
	if err != nil {
		return err
	}

	built := map[string]struct{}{}
	if err := RunBuildWithDependencies(cmd, root, config, serviceNode, built); err != nil {
		return fmt.Errorf("pre-flight build failed for %s/%s: %w", appName, serviceName, err)
	}

	if err := RunCheckWithDependencies(cmd, root, config, serviceNode, built, nil); err != nil {
		return fmt.Errorf("pre-flight check failed for %s/%s: %w", appName, serviceName, err)
	}

	servicePath := filepath.Join(root, "repos", "apps", appName, "services", serviceName)
	containTask, err := ReadRepoCommand(fs, servicePath, "contain")
	if err != nil {
		return err
	}
	if strings.TrimSpace(containTask) != "" {
		if err := ContainService(cmd, fs, appName, serviceName); err != nil {
			return fmt.Errorf("pre-flight contain failed for %s/%s: %w", appName, serviceName, err)
		}
	}

	return nil
}

func RunService(cmd *cobra.Command, fs afero.Fs, appName string, serviceName string) error {
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
	runTask, err := ReadRepoCommand(fs, servicePath, "run")
	if err != nil {
		return err
	}
	if strings.TrimSpace(runTask) == "" {
		fmt.Fprintf(cmd.OutOrStdout(), "run skipped for service %s/%s: no run task\n", resolvedApp, resolvedSvc)
		return nil
	}

	if err := PreflightService(cmd, fs, root, config, resolvedApp, resolvedSvc); err != nil {
		return err
	}

	execCmd := exec.Command("bash", "-lc", runTask)
	execCmd.Dir = servicePath
	execCmd.Stdout = cmd.OutOrStdout()
	execCmd.Stderr = cmd.ErrOrStderr()
	execCmd.Stdin = cmd.InOrStdin()
	return execCmd.Run()
}

func RunApp(cmd *cobra.Command, fs afero.Fs, appName string) error {
	root, err := EnsureWorkspaceRoot(fs)
	if err != nil {
		return err
	}

	config, err := ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}

	appIdx := FindAppIndex(config, appName)
	if appIdx == -1 {
		return fmt.Errorf("app not found: %s", appName)
	}

	appPath := filepath.Join(root, "repos", "apps", appName)
	runTask, err := ReadRepoCommand(fs, appPath, "run")
	if err != nil {
		return err
	}
	if strings.TrimSpace(runTask) == "" {
		fmt.Fprintf(cmd.OutOrStdout(), "run skipped for app %s: no run task\n", appName)
		return nil
	}

	for _, svc := range config.Apps[appIdx].Services {
		if err := PreflightService(cmd, fs, root, config, appName, svc.Name); err != nil {
			return err
		}
	}

	execCmd := exec.Command("bash", "-lc", runTask)
	execCmd.Dir = appPath
	execCmd.Stdout = cmd.OutOrStdout()
	execCmd.Stderr = cmd.ErrOrStderr()
	execCmd.Stdin = cmd.InOrStdin()
	return execCmd.Run()
}
