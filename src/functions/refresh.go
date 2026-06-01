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
	keys, err := SortDependencyGraph(graph)
	if err != nil {
		return err
	}
	order := make([]Node, 0, len(keys))
	for _, key := range keys {
		node, ok := index[key]
		if !ok {
			return fmt.Errorf("dependency node missing: %s", key)
		}
		order = append(order, node)
	}
	appNames := make([]string, 0, len(config.Apps))
	for _, app := range config.Apps {
		appNames = append(appNames, app.Name)
	}
	return runRefreshNodes(cmd, fs, root, config, order, appNames, true)
}

// runRefreshNodes clones (if missing), installs, builds, and checks the given nodes
// in the order provided (which the caller has already topologically sorted, so every
// dependency precedes the node that needs it). appShells names registered apps to clone
// by remote even though they contribute no nodes (an app with no declared
// services/libraries/tests) — deduped against the node clones, so an app already cloned
// via one of its components is not cloned twice. When cloneNonCode is set it also clones
// the workspace's non-code repos (infra/docs) — a whole-workspace concern a scoped run
// deliberately skips.
func runRefreshNodes(cmd *cobra.Command, fs afero.Fs, root string, config WorkspaceConfig, order []Node, appShells []string, cloneNonCode bool) error {
	if len(order) == 0 && len(appShells) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "refresh skipped: no workspace repositories")
		return nil
	}

	cloned := map[string]struct{}{}
	for _, node := range order {
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

	if err := cloneAppShellsIfMissing(cmd, fs, root, config, appShells, cloned); err != nil {
		return err
	}

	if cloneNonCode {
		if err := cloneNonCodeReposIfMissing(cmd, fs, root, config); err != nil {
			return err
		}
	}

	for _, node := range order {
		label, path, _, err := NodeBuildInfo(root, node)
		if err != nil {
			return err
		}
		if err := runInstall(cmd, fs, label, path); err != nil {
			return err
		}
	}

	for _, node := range order {
		if err := buildNode(cmd, root, config, node); err != nil {
			return err
		}
	}

	for _, node := range order {
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

// buildNode builds a single node and records its build hash — the same effect
// RunBuildWithDependencies has for its target, but without re-walking dependencies.
// The caller has already placed every dependency earlier in the order, and a
// --no-deps scoped run intentionally omits them, so building the node alone here is
// correct for both the whole-workspace and scoped paths.
func buildNode(cmd *cobra.Command, root string, config WorkspaceConfig, node Node) error {
	label, path, hashPath, err := NodeBuildInfo(root, node)
	if err != nil {
		return err
	}
	if err := RunBuild(cmd, label, path, hashPath, root); err != nil {
		return err
	}
	if treeHash, err := CalcNodeTreeHash(afero.NewOsFs(), root, config, node); err == nil {
		_ = SetManifestBuildHash(afero.NewOsFs(), root, nodeKey(node), treeHash)
	}
	return nil
}

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

func cloneNonCodeReposIfMissing(cmd *cobra.Command, fs afero.Fs, root string, config WorkspaceConfig) error {
	for _, entry := range config.Infrastructure {
		dest := filepath.Join(root, tokens.RepoDirRoot, tokens.RepoDirInfra, entry.Name)
		if err := cloneRepoIfMissing(cmd, fs, root, dest, entry.Name, entry.Remote); err != nil {
			return err
		}
	}
	for _, entry := range config.Docs {
		dest := filepath.Join(root, tokens.RepoDirRoot, tokens.RepoDirDocs, entry.Name)
		if err := cloneRepoIfMissing(cmd, fs, root, dest, entry.Name, entry.Remote); err != nil {
			return err
		}
	}
	return nil
}

// cloneAppShellsIfMissing clones each named app by its remote when the app directory is
// missing — covering apps registered in the workspace that declare no
// services/libraries/tests and so contribute no graph nodes. The cloned set is shared
// with the node-clone pass so an app already cloned via one of its components is skipped.
func cloneAppShellsIfMissing(cmd *cobra.Command, fs afero.Fs, root string, config WorkspaceConfig, appNames []string, cloned map[string]struct{}) error {
	for _, name := range appNames {
		idx := FindAppIndex(config, name)
		if idx == -1 {
			continue
		}
		dest := filepath.Join(root, tokens.RepoDirRoot, tokens.RepoDirApps, name)
		if _, done := cloned[dest]; done {
			continue
		}
		cloned[dest] = struct{}{}
		if err := cloneRepoIfMissing(cmd, fs, root, dest, name, config.Apps[idx].Remote); err != nil {
			return err
		}
	}
	return nil
}

// cloneRepoIfMissing clones a single repo (at dest, named name, from remote) when its
// directory is missing. A repo with no remote (or remote "none") is skipped with a note
// rather than failing. Used for non-code repos and for app shells alike.
func cloneRepoIfMissing(cmd *cobra.Command, fs afero.Fs, root string, dest string, name string, remote string) error {
	if info, statErr := fs.Stat(dest); statErr == nil && info.IsDir() {
		return nil
	}
	if remote == "" || remote == tokens.RemoteNone {
		fmt.Fprintf(cmd.OutOrStdout(), "skipping clone of %s: no remote configured\n", name)
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "cloning %s\n", name)
	return RunWorkspaceTaskAt(fs, root, cmd.OutOrStdout(), cmd.ErrOrStderr(), tokens.WorkspaceTaskClone, name, "")
}

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
