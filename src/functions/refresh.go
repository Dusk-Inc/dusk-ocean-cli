package functions

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

func RunRefresh(cmd *cobra.Command, fs afero.Fs, root string, config WorkspaceConfig) error {
	graph, index, err := BuildWorkspaceGraph(config)
	if err != nil {
		return err
	}
	order, err := SortDependencyGraph(graph)
	if err != nil {
		return err
	}
	if len(order) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "refresh skipped: no workspace repositories")
		return nil
	}

	for _, key := range order {
		node, ok := index[key]
		if !ok {
			return fmt.Errorf("dependency node missing: %s", key)
		}
		label, path, _, err := NodeBuildInfo(root, node)
		if err != nil {
			return err
		}
		if err := runInstall(cmd, fs, label, path); err != nil {
			return err
		}
	}

	built := map[string]struct{}{}
	for _, key := range order {
		node, ok := index[key]
		if !ok {
			return fmt.Errorf("dependency node missing: %s", key)
		}
		if err := RunBuildWithDependencies(cmd, root, config, node, built); err != nil {
			return err
		}
	}

	for _, key := range order {
		node, ok := index[key]
		if !ok {
			return fmt.Errorf("dependency node missing: %s", key)
		}
		label, path, hashPath, err := NodeCheckInfo(root, node)
		if err != nil {
			return err
		}
		if err := RunCheck(cmd, label, path, hashPath, nil, root); err != nil {
			return err
		}
	}

	return nil
}

// RunInstall resolves a repo by name and runs its install task (REQ 6.1/6.2).
func RunInstall(cmd *cobra.Command, fs afero.Fs, repoName string) error {
	root, err := EnsureWorkspaceRoot(fs)
	if err != nil {
		return err
	}
	config, err := ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	target, err := ResolveTargetByName(config, root, repoName)
	if err != nil {
		return err
	}
	label := FormatTargetLabel(target)
	return runInstall(cmd, fs, label, target.Path)
}

func runInstall(cmd *cobra.Command, fs afero.Fs, label string, targetPath string) error {
	installCmd, err := ReadRepoCommand(fs, targetPath, "install")
	if err != nil {
		return err
	}
	if strings.TrimSpace(installCmd) == "" {
		fmt.Fprintf(cmd.OutOrStdout(), "install skipped for %s: no install command\n", label)
		return nil
	}
	execCmd := exec.Command("bash", "-lc", installCmd)
	execCmd.Dir = targetPath
	execCmd.Stdout = cmd.OutOrStdout()
	execCmd.Stderr = cmd.ErrOrStderr()
	return execCmd.Run()
}
