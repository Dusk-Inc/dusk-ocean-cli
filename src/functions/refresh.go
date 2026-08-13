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

// RunRefresh refreshes the whole workspace: it builds the dependency graph, sorts it so
// every dependency precedes its dependents, and clones, installs, builds, and checks each
// node in that order. Apps that declare no services, libraries, or tests contribute no
// graph nodes, so they are passed separately as shells to be cloned by remote.
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
//
// It runs in two passes. The clone pass tolerates access failures, recording a per-node
// status so an inaccessible repo is reported rather than aborting the whole refresh. The
// missing-deps pass then skips any node whose local dependency is unavailable — itself not
// OK, or absent on disk — and records what it was missing; the topological order is what
// guarantees a dependency is classified before anything that needs it.
func runRefreshNodes(cmd *cobra.Command, fs afero.Fs, root string, config WorkspaceConfig, order []Node, appShells []string, cloneNonCode bool) error {
	if len(order) == 0 && len(appShells) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "refresh skipped: no workspace repositories")
		return nil
	}

	index := make(map[string]Node, len(order))
	keys := make([]string, 0, len(order))
	for _, node := range order {
		key := nodeKey(node)
		index[key] = node
		keys = append(keys, key)
	}

	statusByKey := map[string]RefreshNodeStatus{}
	missingByKey := map[string][]string{}
	cloneStatusByDest := map[string]RefreshNodeStatus{}
	cloned := map[string]struct{}{}

	for _, key := range keys {
		node := index[key]
		dest, _, err := resolveNodeCloneTarget(root, config, node)
		if err != nil {
			return err
		}
		cloned[dest] = struct{}{}
		if cached, done := cloneStatusByDest[dest]; done {
			statusByKey[key] = cached
			continue
		}
		statusByKey[key] = attemptCloneForRefresh(cmd, fs, root, config, node, dest)
		cloneStatusByDest[dest] = statusByKey[key]
	}

	if err := cloneAppShellsIfMissing(cmd, fs, root, config, appShells, cloned); err != nil {
		return err
	}

	if cloneNonCode {
		if err := cloneNonCodeReposIfMissing(cmd, fs, root, config); err != nil {
			return err
		}
	}

	for _, key := range keys {
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

	for _, key := range keys {
		if statusByKey[key] != RefreshStatusOK {
			continue
		}
		label, path, _, err := NodeBuildInfo(root, index[key])
		if err != nil {
			return err
		}
		if err := runInstall(cmd, fs, label, path); err != nil {
			return err
		}
	}

	for _, key := range keys {
		if statusByKey[key] != RefreshStatusOK {
			continue
		}
		if err := buildNode(cmd, root, config, index[key]); err != nil {
			return err
		}
	}

	for _, key := range keys {
		if statusByKey[key] != RefreshStatusOK {
			continue
		}
		label, path, hashPath, err := NodeCheckInfo(root, index[key])
		if err != nil {
			return err
		}
		if err := RunCheck(cmd, label, path, hashPath, nil, root); err != nil {
			return err
		}
	}

	report := buildRefreshReport(keys, index, statusByKey, missingByKey)
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

// buildRefreshReport assembles a per-node report from the recorded statuses,
// ordered by key for stable output.
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

// refreshNodeLabel returns the human-readable label for a node, qualified by its kind and
// (for app-scoped kinds) its app, as the refresh report prints it.
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

// RunInstall installs one repo by name, resolving it against the workspace config from the
// discovered workspace root. Unlike RunRefresh it touches no dependency graph — it is the
// single-target install, leaving the caller to decide whether dependencies matter.
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

// resolveNodeCloneTarget returns the directory a node's repo is cloned into and the name the
// clone workspace-task is invoked with, erroring when the node names something the workspace
// config does not register. The two differ for app-scoped kinds: several nodes share one app
// repo, so dest is the app directory while taskTarget is the app itself.
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

// cloneNonCodeReposIfMissing clones the registered repos that contribute no dependency-graph
// nodes — templates, infrastructure, and docs. Neither the node-clone pass nor
// cloneAppShellsIfMissing reaches them, so a kind omitted here is a kind refresh silently
// never clones.
func cloneNonCodeReposIfMissing(cmd *cobra.Command, fs afero.Fs, root string, config WorkspaceConfig) error {
	for _, entry := range config.Templates {
		cloneNonCodeRepoOrSkip(cmd, fs, root, tokens.RepoDirTemplates, entry.Name, entry.Remote)
	}
	for _, entry := range config.Infrastructure {
		cloneNonCodeRepoOrSkip(cmd, fs, root, tokens.RepoDirInfra, entry.Name, entry.Remote)
	}
	for _, entry := range config.Docs {
		cloneNonCodeRepoOrSkip(cmd, fs, root, tokens.RepoDirDocs, entry.Name, entry.Remote)
	}
	return nil
}

// cloneNonCodeRepoOrSkip clones one non-code repo into dir, downgrading a clone failure to a
// skip notice the way attemptCloneForRefresh does for graph nodes. One unreachable or
// malformed remote must not abort the refresh and strand every repo queued behind it.
func cloneNonCodeRepoOrSkip(cmd *cobra.Command, fs afero.Fs, root string, dir string, name string, remote string) {
	dest := filepath.Join(root, tokens.RepoDirRoot, dir, name)
	if err := cloneRepoIfMissing(cmd, fs, root, dest, name, remote); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "skip %s: clone failed (%v)\n", dest, err)
	}
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

// cloneNodeRepoIfMissing clones the repo backing a graph node when its directory is absent.
// It is the node-keyed counterpart to cloneRepoIfMissing, resolving the destination and task
// target from the node rather than taking them from the caller.
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

// runInstall runs a target's configured install command in its own directory, streaming
// output through cmd. A target that configures no install command is skipped with a notice
// rather than treated as an error, since not every repo has one.
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
