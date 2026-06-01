package functions

import (
	"fmt"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// RunScopedRefresh clones (if missing), installs, builds, and checks the repo named
// by repo and, unless noDeps is set, the transitive closure of its dependencies, in
// dependency order. Unlike a whole-workspace refresh it never touches the non-code
// repos (infra/docs), which are not part of any code repo's dependency graph.
func RunScopedRefresh(cmd *cobra.Command, fs afero.Fs, root string, config WorkspaceConfig, repo string, noDeps bool) error {
	order, err := ScopedRefreshOrder(config, repo, noDeps)
	if err != nil {
		return err
	}
	return runRefreshNodes(cmd, fs, root, config, order, scopedAppShells(config, repo), false)
}

// scopedAppShells returns the apps a scoped run must clone by remote even if they
// contribute no graph nodes: the --repo target when it names an app. An app that does
// declare components is cloned via those nodes (and deduped), so naming it here is
// harmless; an empty app shell is cloned only because it is named here.
func scopedAppShells(config WorkspaceConfig, repo string) []string {
	if FindAppIndex(config, repo) != -1 {
		return []string{repo}
	}
	return nil
}

// ScopedRefreshOrder resolves the nodes a scoped refresh must process, in dependency
// (topological) order: the repo named by repo plus, unless noDeps is set, the
// transitive closure of its dependencies. It returns an error naming the repo if it
// is absent from the workspace config, or a cycle error if the targeted repos form a
// dependency cycle.
func ScopedRefreshOrder(config WorkspaceConfig, repo string, noDeps bool) ([]Node, error) {
	graph, index, err := BuildWorkspaceGraph(config)
	if err != nil {
		return nil, err
	}
	seeds, err := scopedSeedKeys(config, repo)
	if err != nil {
		return nil, err
	}
	inScope := collectScope(graph, seeds, noDeps)
	keys, err := SortDependencyGraph(subgraphFor(graph, inScope))
	if err != nil {
		return nil, err
	}
	order := make([]Node, 0, len(keys))
	for _, key := range keys {
		node, ok := index[key]
		if !ok {
			return nil, fmt.Errorf("dependency node missing: %s", key)
		}
		order = append(order, node)
	}
	return order, nil
}

// scopedSeedKeys resolves a workspace repo name to the graph keys that belong to it:
// a global library or a project is a single node; an app contributes all of its
// services, libraries, and tests. It errors only when the name matches no repo in the
// config at all; a name that DOES match — e.g. an app that exists but declares no
// services/libraries/tests — resolves to an empty seed set (a valid, empty scope), not
// an error, since "absent from config" and "present but has no buildable components"
// are different conditions.
func scopedSeedKeys(config WorkspaceConfig, repo string) ([]string, error) {
	matched := false
	var seeds []string
	if FindGlobalLibraryIndex(config, repo) != -1 {
		matched = true
		seeds = append(seeds, GlobalLibKey(repo))
	}
	if FindProjectIndex(config, repo) != -1 {
		matched = true
		seeds = append(seeds, ProjectKey(repo))
	}
	if idx := FindAppIndex(config, repo); idx != -1 {
		matched = true
		app := config.Apps[idx]
		for _, service := range app.Services {
			seeds = append(seeds, ServiceKey(app.Name, service.Name))
		}
		for _, lib := range app.Libraries {
			seeds = append(seeds, AppLibKey(app.Name, lib.Name))
		}
		for _, test := range app.Testing {
			seeds = append(seeds, fmt.Sprintf("test:%s:%s", app.Name, test.Name))
		}
	}
	if !matched {
		return nil, fmt.Errorf("repo %q not found in workspace config", repo)
	}
	return seeds, nil
}

// collectScope returns the set of graph keys in scope: the seeds, plus — unless
// noDeps is set — every key reachable from a seed along the dependency edges.
func collectScope(graph map[string][]string, seeds []string, noDeps bool) map[string]struct{} {
	inScope := make(map[string]struct{}, len(seeds))
	for _, seed := range seeds {
		inScope[seed] = struct{}{}
	}
	if noDeps {
		return inScope
	}
	stack := append([]string{}, seeds...)
	for len(stack) > 0 {
		key := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, dep := range graph[key] {
			if _, ok := inScope[dep]; !ok {
				inScope[dep] = struct{}{}
				stack = append(stack, dep)
			}
		}
	}
	return inScope
}

// subgraphFor restricts a dependency graph to the in-scope keys, keeping only edges
// whose endpoints are both in scope. This makes the topological sort — and its cycle
// detection — operate over the targeted repos alone.
func subgraphFor(graph map[string][]string, inScope map[string]struct{}) map[string][]string {
	sub := make(map[string][]string, len(inScope))
	for key := range inScope {
		deps := []string{}
		for _, dep := range graph[key] {
			if _, ok := inScope[dep]; ok {
				deps = append(deps, dep)
			}
		}
		sub[key] = deps
	}
	return sub
}
