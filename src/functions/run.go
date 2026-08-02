package functions

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/models"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// repoVariableContext builds the token-substitution context for any repo kind
// (mirrors projectVariableContext, generalized to service/app group resolution).
func repoVariableContext(fs afero.Fs, root string, config WorkspaceConfig, kind, appName, repoName string) (VariableContext, error) {
	envValues, err := LoadEnvFile(fs, root, discardWriter{})
	if err != nil {
		return VariableContext{}, err
	}
	env := map[string]string{}
	for _, kv := range os.Environ() {
		if eq := strings.IndexByte(kv, '='); eq > 0 {
			env[kv[:eq]] = kv[eq+1:]
		}
	}
	for k, v := range envValues {
		env[k] = v
	}
	repoVars, err := BuildRepoVariables(config, kind, appName, repoName)
	if err != nil {
		return VariableContext{}, err
	}
	return VariableContext{
		Env:   env,
		Var:   LoadWorkspaceVariables(config),
		Ocean: map[string]string{},
		Repo:  repoVars,
	}, nil
}

// resolveRepoTask reads a repo's config and resolves a lifecycle task through the
// group-override machinery, so services and apps honor --group exactly as projects do.
func resolveRepoTask(fs afero.Fs, root string, config WorkspaceConfig, repoPath, kind, appName, repoName, task string, selection models.GroupSelection) (models.ResolvedCommand, error) {
	repoConfig, err := ReadRepoConfig(fs, repoPath)
	if err != nil {
		return models.ResolvedCommand{}, err
	}
	groups, err := ValidateOverrides(repoConfig)
	if err != nil {
		return models.ResolvedCommand{}, err
	}
	ctx, err := repoVariableContext(fs, root, config, kind, appName, repoName)
	if err != nil {
		return models.ResolvedCommand{}, err
	}
	return ResolveGroupCommand(task, selection, repoConfig, groups, ctx)
}

func PreflightService(cmd *cobra.Command, fs afero.Fs, root string, config WorkspaceConfig, appName string, serviceName string, skipCheck bool) error {
	serviceNode, err := MakeServiceNode(config, appName, serviceName)
	if err != nil {
		return err
	}

	built := map[string]struct{}{}
	if err := RunBuildWithDependencies(cmd, root, config, serviceNode, built); err != nil {
		return fmt.Errorf("pre-flight build failed for %s/%s: %w", appName, serviceName, err)
	}

	if skipCheck {
		fmt.Fprintf(cmd.OutOrStdout(), "pre-flight check skipped for %s/%s (--skip-check)\n", appName, serviceName)
	} else if err := RunCheckWithDependencies(cmd, root, config, serviceNode, built, nil); err != nil {
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

func RunService(cmd *cobra.Command, fs afero.Fs, appName string, serviceName string, skipCheck bool, selection models.GroupSelection) error {
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
	resolved, err := resolveRepoTask(fs, root, config, servicePath, tokens.RepoKindService, resolvedApp, resolvedSvc, "run", selection)
	if err != nil {
		return err
	}
	runTask := resolved.Command
	if strings.TrimSpace(runTask) == "" {
		fmt.Fprintf(cmd.OutOrStdout(), "run skipped for service %s/%s: no run task\n", resolvedApp, resolvedSvc)
		return nil
	}

	// A --group run is a distinct lifecycle mode (e.g. `test` stands up ephemeral deps);
	// it runs the group command directly, skipping the service's build/check/contain preflight.
	if selection.IsBase {
		if err := PreflightService(cmd, fs, root, config, resolvedApp, resolvedSvc, skipCheck); err != nil {
			return err
		}
	}

	envFile, err := LoadEnvFile(fs, root, cmd.OutOrStdout())
	if err != nil {
		return err
	}

	execCmd := exec.Command("bash", "-lc", runTask)
	execCmd.Dir = servicePath
	execCmd.Env = mergeEnvForExec(envFile)
	execCmd.Stdout = cmd.OutOrStdout()
	execCmd.Stderr = cmd.ErrOrStderr()
	execCmd.Stdin = cmd.InOrStdin()
	return execCmd.Run()
}

func RunApp(cmd *cobra.Command, fs afero.Fs, appName string, skipCheck bool, selection models.GroupSelection) error {
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
	resolved, err := resolveRepoTask(fs, root, config, appPath, tokens.RepoKindApp, appName, appName, "run", selection)
	if err != nil {
		return err
	}
	runTask := resolved.Command
	if strings.TrimSpace(runTask) == "" {
		fmt.Fprintf(cmd.OutOrStdout(), "run skipped for app %s: no run task\n", appName)
		return nil
	}

	// A --group run is a distinct lifecycle mode; it runs the group command directly,
	// skipping the per-service build/check/contain preflight.
	if selection.IsBase {
		for _, svc := range config.Apps[appIdx].Services {
			if err := PreflightService(cmd, fs, root, config, appName, svc.Name, skipCheck); err != nil {
				return err
			}
		}
	}

	envFile, err := LoadEnvFile(fs, root, cmd.OutOrStdout())
	if err != nil {
		return err
	}

	execCmd := exec.Command("bash", "-lc", runTask)
	execCmd.Dir = appPath
	execCmd.Env = mergeEnvForExec(envFile)
	execCmd.Stdout = cmd.OutOrStdout()
	execCmd.Stderr = cmd.ErrOrStderr()
	execCmd.Stdin = cmd.InOrStdin()
	return execCmd.Run()
}

func mergeEnvForExec(envFile map[string]string) []string {
	base := os.Environ()
	seen := make(map[string]struct{}, len(base))
	for _, kv := range base {
		if eq := strings.IndexByte(kv, '='); eq > 0 {
			seen[kv[:eq]] = struct{}{}
		}
	}
	out := make([]string, 0, len(base)+len(envFile))
	out = append(out, base...)
	for k, v := range envFile {
		if _, exists := seen[k]; !exists {
			out = append(out, k+"="+v)
		}
	}
	return out
}
