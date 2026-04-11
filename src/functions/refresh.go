package functions

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
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

	cloned := map[string]struct{}{}
	for _, key := range order {
		node, ok := index[key]
		if !ok {
			return fmt.Errorf("dependency node missing: %s", key)
		}
		dest, _, err := resolveNodeCloneTarget(root, config, node)
		if err != nil {
			return err
		}
		if _, done := cloned[dest]; !done {
			cloned[dest] = struct{}{}
			if err := cloneNodeRepoIfMissing(cmd, fs, root, config, node); err != nil {
				return err
			}
		}
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

// resolveNodeCloneTarget returns the expected on-disk path and the target
// name to use with the workspace "clone" task for the given node. For app
// sub-repos (services, app libraries, app tests) the clone target is the
// parent app directory, since all sub-repos share a single git history.
func resolveNodeCloneTarget(root string, config WorkspaceConfig, node Node) (dest string, taskTarget string, err error) {
	switch node.Kind {
	case NodeAppLib, NodeService, NodeAppTest:
		if FindAppIndex(config, node.App) == -1 {
			return "", "", fmt.Errorf("app not found in workspace: %s", node.App)
		}
		dest = filepath.Join(root, tokens.RepoDirRoot, tokens.RepoDirApps, node.App)
		return dest, node.App, nil
	case NodeGlobalLib:
		if FindGlobalLibraryIndex(config, node.Name) == -1 {
			return "", "", fmt.Errorf("library not found in workspace: %s", node.Name)
		}
		dest = filepath.Join(root, tokens.RepoDirRoot, tokens.RepoDirLibs, node.Name)
		return dest, node.Name, nil
	case NodeProject:
		if FindProjectIndex(config, node.Name) == -1 {
			return "", "", fmt.Errorf("project not found in workspace: %s", node.Name)
		}
		dest = filepath.Join(root, tokens.RepoDirRoot, tokens.RepoDirProjects, node.Name)
		return dest, node.Name, nil
	}
	return "", "", fmt.Errorf("unsupported node kind: %v", node.Kind)
}

// cloneNodeRepoIfMissing runs the workspace "clone" task for the repo that
// owns node if its destination directory does not already exist. If the
// directory is already present the function is a no-op.
func cloneNodeRepoIfMissing(cmd *cobra.Command, fs afero.Fs, root string, config WorkspaceConfig, node Node) error {
	dest, taskTarget, err := resolveNodeCloneTarget(root, config, node)
	if err != nil {
		return err
	}
	if info, statErr := fs.Stat(dest); statErr == nil && info.IsDir() {
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "cloning %s\n", taskTarget)
	return RunWorkspaceTaskAt(fs, root, cmd.OutOrStdout(), cmd.ErrOrStderr(), tokens.WorkspaceTaskClone, taskTarget, "")
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
