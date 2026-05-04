package functions

import (
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

type RefreshNodeStatus string

const (
	RefreshStatusOK          RefreshNodeStatus = "ok"
	RefreshStatusNoAccess    RefreshNodeStatus = "no-access"
	RefreshStatusMissingDeps RefreshNodeStatus = "missing-deps"
)

type RefreshNodeReport struct {
	Key     string
	Label   string
	Status  RefreshNodeStatus
	Missing []string
}

type RefreshReport struct {
	Entries []RefreshNodeReport
}

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

	statusByKey := map[string]RefreshNodeStatus{}
	missingByKey := map[string][]string{}
	cloneStatusByDest := map[string]RefreshNodeStatus{}

	for _, key := range order {
		node, ok := index[key]
		if !ok {
			return fmt.Errorf("dependency node missing: %s", key)
		}
		dest, _, err := resolveNodeCloneTarget(root, config, node)
		if err != nil {
			return err
		}
		if cached, done := cloneStatusByDest[dest]; done {
			statusByKey[key] = cached
			continue
		}
		statusByKey[key] = attemptCloneForRefresh(cmd, fs, root, config, node, dest)
		cloneStatusByDest[dest] = statusByKey[key]
	}

	for _, key := range order {
		if statusByKey[key] != RefreshStatusOK {
			continue
		}
		node := index[key]
		var missing []string
		for _, dep := range node.Deps {
			depNode, err := resolveDependencyNode(config, node, dep)
			if err != nil {
				missing = append(missing, strings.TrimSpace(dep.Lib))
				continue
			}
			depKey := nodeKey(depNode)
			if depStatus, known := statusByKey[depKey]; known && depStatus != RefreshStatusOK {
				missing = append(missing, depKey)
				continue
			}
			_, depPath, _, err := NodeBuildInfo(root, depNode)
			if err != nil {
				missing = append(missing, depKey)
				continue
			}
			if !DirExists(fs, depPath) {
				missing = append(missing, depKey)
			}
		}
		if len(missing) > 0 {
			statusByKey[key] = RefreshStatusMissingDeps
			missingByKey[key] = missing
		}
	}

	for _, key := range order {
		if statusByKey[key] != RefreshStatusOK {
			continue
		}
		node := index[key]
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
		if statusByKey[key] != RefreshStatusOK {
			continue
		}
		node := index[key]
		label, path, hashPath, err := NodeBuildInfo(root, node)
		if err != nil {
			return err
		}
		if err := RunBuild(cmd, label, path, hashPath, root); err != nil {
			return err
		}
		built[key] = struct{}{}
	}

	for _, key := range order {
		if statusByKey[key] != RefreshStatusOK {
			continue
		}
		node := index[key]
		label, path, hashPath, err := NodeCheckInfo(root, node)
		if err != nil {
			return err
		}
		if err := RunCheck(cmd, label, path, hashPath, nil, root); err != nil {
			return err
		}
	}

	report := buildRefreshReport(order, index, statusByKey, missingByKey)
	WriteRefreshReport(cmd.OutOrStdout(), report)
	return nil
}

// attemptCloneForRefresh resolves the clone target for node; if the local
// directory already exists it returns OK without invoking the clone task,
// otherwise it runs the workspace clone task and returns NoAccess if the
// clone command fails (or the target directory is still absent after).
func attemptCloneForRefresh(cmd *cobra.Command, fs afero.Fs, root string, config WorkspaceConfig, node Node, dest string) RefreshNodeStatus {
	if info, err := fs.Stat(dest); err == nil && info.IsDir() {
		return RefreshStatusOK
	}
	if err := cloneNodeRepoIfMissing(cmd, fs, root, config, node); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "skip %s: clone failed (%v)\n", dest, err)
		return RefreshStatusNoAccess
	}
	if info, err := fs.Stat(dest); err != nil || !info.IsDir() {
		fmt.Fprintf(cmd.ErrOrStderr(), "skip %s: clone reported success but directory is absent\n", dest)
		return RefreshStatusNoAccess
	}
	return RefreshStatusOK
}

func buildRefreshReport(order []string, index map[string]Node, statusByKey map[string]RefreshNodeStatus, missingByKey map[string][]string) RefreshReport {
	report := RefreshReport{}
	keys := append([]string(nil), order...)
	sort.Strings(keys)
	for _, key := range keys {
		node, ok := index[key]
		if !ok {
			continue
		}
		entry := RefreshNodeReport{
			Key:    key,
			Label:  refreshNodeLabel(node),
			Status: statusByKey[key],
		}
		if entry.Status == RefreshStatusMissingDeps {
			entry.Missing = append(entry.Missing, missingByKey[key]...)
			sort.Strings(entry.Missing)
		}
		report.Entries = append(report.Entries, entry)
	}
	return report
}

func refreshNodeLabel(node Node) string {
	switch node.Kind {
	case NodeService:
		return fmt.Sprintf("service %s/%s", node.App, node.Name)
	case NodeAppLib:
		return fmt.Sprintf("app library %s/%s", node.App, node.Name)
	case NodeAppTest:
		return fmt.Sprintf("test %s/%s", node.App, node.Name)
	case NodeGlobalLib:
		return fmt.Sprintf("global library %s", node.Name)
	case NodeProject:
		return fmt.Sprintf("project %s", node.Name)
	}
	return node.Name
}

// WriteRefreshReport prints a grouped summary of refresh outcomes. Groups
// are emitted only when populated so a fully-clean run prints just the
// installed list. The output is intended to be human-readable; agents and
// scripts consume the manifest hashes for machine state.
func WriteRefreshReport(out io.Writer, report RefreshReport) {
	var ok, noAccess, missing []RefreshNodeReport
	for _, entry := range report.Entries {
		switch entry.Status {
		case RefreshStatusOK:
			ok = append(ok, entry)
		case RefreshStatusNoAccess:
			noAccess = append(noAccess, entry)
		case RefreshStatusMissingDeps:
			missing = append(missing, entry)
		}
	}
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Refresh report")
	fmt.Fprintln(out, "==============")
	fmt.Fprintf(out, "installed: %d  no access: %d  missing dependencies: %d\n",
		len(ok), len(noAccess), len(missing))
	if len(ok) > 0 {
		fmt.Fprintln(out, "\nInstalled:")
		for _, entry := range ok {
			fmt.Fprintf(out, "  - %s\n", entry.Label)
		}
	}
	if len(noAccess) > 0 {
		fmt.Fprintln(out, "\nNo access (clone unavailable):")
		for _, entry := range noAccess {
			fmt.Fprintf(out, "  - %s\n", entry.Label)
		}
	}
	if len(missing) > 0 {
		fmt.Fprintln(out, "\nSkipped due to missing dependencies:")
		for _, entry := range missing {
			fmt.Fprintf(out, "  - %s -> missing: %s\n", entry.Label, strings.Join(entry.Missing, ", "))
		}
	}
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
