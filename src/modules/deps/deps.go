package deps

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/hash"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/workspace"
	"github.com/spf13/cobra"
)

type NodeKind string

const (
	NodeService   NodeKind = "service"
	NodeAppLib    NodeKind = "app-lib"
	NodeGlobalLib NodeKind = "global-lib"
	NodeProject   NodeKind = "project"
)

type Node struct {
	Kind     NodeKind
	App      string
	Name     string
	Deps     []string
}

func RunBuildWithDependencies(cmd *cobra.Command, root string, config workspace.WorkspaceConfig, target Node, built map[string]struct{}) error {
	deps, err := CollectDependencyOrder(config, target)
	if err != nil {
		return err
	}
	for _, dep := range deps {
		key := nodeKey(dep)
		if _, ok := built[key]; ok {
			continue
		}
		label, path, hashPath, err := nodeBuildInfo(root, dep)
		if err != nil {
			return err
		}
		if err := workspace.RunBuild(cmd, label, path, hashPath); err != nil {
			return err
		}
		built[key] = struct{}{}
	}
	label, path, hashPath, err := nodeBuildInfo(root, target)
	if err != nil {
		return err
	}
	return workspace.RunBuild(cmd, label, path, hashPath)
}

func RunCheckWithDependencies(cmd *cobra.Command, root string, config workspace.WorkspaceConfig, target Node, built map[string]struct{}, passThrough []string) error {
	deps, err := CollectDependencyOrder(config, target)
	if err != nil {
		return err
	}
	for _, dep := range deps {
		key := nodeKey(dep)
		if _, ok := built[key]; ok {
			continue
		}
		label, path, hashPath, err := nodeBuildInfo(root, dep)
		if err != nil {
			return err
		}
		if err := workspace.RunBuild(cmd, label, path, hashPath); err != nil {
			return err
		}
		built[key] = struct{}{}
	}
	label, path, hashPath, err := nodeCheckInfo(root, target)
	if err != nil {
		return err
	}
	return workspace.RunCheck(cmd, label, path, hashPath, passThrough, root)
}

func CollectDependencyOrder(config workspace.WorkspaceConfig, target Node) ([]Node, error) {
	visited := map[string]struct{}{}
	stack := map[string]struct{}{}
	var order []Node

	var visit func(node Node) error
	visit = func(node Node) error {
		key := nodeKey(node)
		if _, ok := stack[key]; ok {
			return fmt.Errorf("dependency cycle detected")
		}
		if _, ok := visited[key]; ok {
			return nil
		}
		stack[key] = struct{}{}
		for _, depName := range node.Deps {
			depNode, err := resolveDependencyNode(config, node, depName)
			if err != nil {
				return err
			}
			if err := visit(depNode); err != nil {
				return err
			}
		}
		delete(stack, key)
		visited[key] = struct{}{}
		order = append(order, node)
		return nil
	}

	if err := visit(target); err != nil {
		return nil, err
	}
	if len(order) == 0 {
		return nil, nil
	}
	return order[:len(order)-1], nil
}

func BuildDependencyGraph(config workspace.WorkspaceConfig) (map[string][]string, error) {
	graph := map[string][]string{}
	globalLibs := map[string]string{}
	appLibs := map[string]map[string]string{}
	projects := map[string]string{}

	for _, lib := range config.Libraries {
		key := GlobalLibKey(lib.Name)
		if _, ok := globalLibs[lib.Name]; ok {
			return nil, fmt.Errorf("global library name collision: %s", lib.Name)
		}
		globalLibs[lib.Name] = key
		graph[key] = []string{}
	}

	for _, app := range config.Apps {
		if _, ok := appLibs[app.Name]; !ok {
			appLibs[app.Name] = map[string]string{}
		}
		for _, lib := range app.Libraries {
			key := AppLibKey(app.Name, lib.Name)
			appLibs[app.Name][lib.Name] = key
			graph[key] = []string{}
		}
	}

	for _, project := range config.Projects {
		key := ProjectKey(project.Name)
		if _, ok := projects[project.Name]; ok {
			return nil, fmt.Errorf("project name collision: %s", project.Name)
		}
		projects[project.Name] = key
		graph[key] = []string{}
	}

	for _, lib := range config.Libraries {
		key := GlobalLibKey(lib.Name)
		graph[key] = append([]string{}, resolveGlobalDeps(lib.Deps, globalLibs)...)
	}

	for _, app := range config.Apps {
		for _, lib := range app.Libraries {
			key := AppLibKey(app.Name, lib.Name)
			graph[key] = append([]string{}, resolveAppDeps(app.Name, lib.Deps, appLibs, globalLibs, projects)...)
		}
	}

	for _, project := range config.Projects {
		key := ProjectKey(project.Name)
		graph[key] = append([]string{}, resolveGlobalDeps(project.Deps, globalLibs)...)
	}

	return graph, nil
}

func HasPath(graph map[string][]string, start string, target string) bool {
	if start == target {
		return true
	}
	seen := map[string]struct{}{}
	stack := []string{start}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if node == target {
			return true
		}
		if _, ok := seen[node]; ok {
			continue
		}
		seen[node] = struct{}{}
		for _, next := range graph[node] {
			if _, ok := seen[next]; !ok {
				stack = append(stack, next)
			}
		}
	}
	return false
}

func GlobalLibKey(name string) string {
	return fmt.Sprintf("lib:global:%s", name)
}

func AppLibKey(appName string, name string) string {
	return fmt.Sprintf("lib:app:%s:%s", appName, name)
}

func ProjectKey(name string) string {
	return fmt.Sprintf("project:%s", name)
}

func ServiceKey(appName string, name string) string {
	return fmt.Sprintf("service:%s:%s", appName, name)
}

func nodeKey(node Node) string {
	switch node.Kind {
	case NodeService:
		return ServiceKey(node.App, node.Name)
	case NodeAppLib:
		return AppLibKey(node.App, node.Name)
	case NodeGlobalLib:
		return GlobalLibKey(node.Name)
	case NodeProject:
		return ProjectKey(node.Name)
	default:
		return ""
	}
}

func resolveDependencyNode(config workspace.WorkspaceConfig, target Node, depName string) (Node, error) {
	depName = strings.TrimSpace(depName)
	if depName == "" {
		return Node{}, fmt.Errorf("dependency name is required")
	}

	if target.Kind == NodeGlobalLib || target.Kind == NodeProject {
		lib, ok, err := findGlobalLibraryByName(config, depName)
		if err != nil {
			return Node{}, err
		}
		if !ok {
			return Node{}, fmt.Errorf("dependency not found: %s", depName)
		}
		return Node{
			Kind:     NodeGlobalLib,
			Name:     lib.Name,
			Deps:     lib.Deps,
		}, nil
	}

	var candidates []Node

	if target.App != "" {
		if lib, ok := findAppLibraryByName(config, target.App, depName); ok {
			candidates = append(candidates, Node{
				Kind:     NodeAppLib,
				App:      target.App,
				Name:     lib.Name,
				Deps:     lib.Deps,
			})
		}
	}

	if lib, ok, err := findGlobalLibraryByName(config, depName); err != nil {
		return Node{}, err
	} else if ok {
		candidates = append(candidates, Node{
			Kind:     NodeGlobalLib,
			Name:     lib.Name,
			Deps:     lib.Deps,
		})
	}

	if project, ok, err := findProjectByName(config, depName); err != nil {
		return Node{}, err
	} else if ok {
		candidates = append(candidates, Node{
			Kind:     NodeProject,
			Name:     project.Name,
			Deps:     project.Deps,
		})
	}

	if len(candidates) == 0 {
		return Node{}, fmt.Errorf("dependency not found: %s", depName)
	}
	if len(candidates) > 1 {
		return Node{}, fmt.Errorf("dependency name is ambiguous: %s", depName)
	}
	return candidates[0], nil
}

func nodeBuildInfo(root string, node Node) (string, string, string, error) {
	switch node.Kind {
	case NodeService:
		path := filepath.Join(root, "repos", "apps", node.App, "services", node.Name)
		label := fmt.Sprintf("service %s/%s", node.App, node.Name)
		hashPath := hash.MakeHashPath(root, "services", node.App, node.Name)
		return label, path, hashPath, nil
	case NodeAppLib:
		path := filepath.Join(root, "repos", "apps", node.App, "libs", node.Name)
		label := fmt.Sprintf("library %s/%s", node.App, node.Name)
		hashPath := hash.MakeHashPath(root, "libs", node.App, node.Name)
		return label, path, hashPath, nil
	case NodeGlobalLib:
		path := filepath.Join(root, "repos", "libs", node.Name)
		label := fmt.Sprintf("library %s", node.Name)
		hashPath := hash.MakeHashPath(root, "libs", "global", node.Name)
		return label, path, hashPath, nil
	case NodeProject:
		path := filepath.Join(root, "repos", "projects", node.Name)
		label := fmt.Sprintf("project %s", node.Name)
		hashPath := hash.MakeHashPath(root, "projects", node.Name)
		return label, path, hashPath, nil
	default:
		return "", "", "", fmt.Errorf("unsupported dependency node")
	}
}

func nodeCheckInfo(root string, node Node) (string, string, string, error) {
	switch node.Kind {
	case NodeService:
		path := filepath.Join(root, "repos", "apps", node.App, "services", node.Name)
		label := fmt.Sprintf("service %s/%s", node.App, node.Name)
		hashPath := hash.MakeCheckHashPath(root, "services", node.App, node.Name)
		return label, path, hashPath, nil
	case NodeAppLib:
		path := filepath.Join(root, "repos", "apps", node.App, "libs", node.Name)
		label := fmt.Sprintf("library %s/%s", node.App, node.Name)
		hashPath := hash.MakeCheckHashPath(root, "libs", node.App, node.Name)
		return label, path, hashPath, nil
	case NodeGlobalLib:
		path := filepath.Join(root, "repos", "libs", node.Name)
		label := fmt.Sprintf("library %s", node.Name)
		hashPath := hash.MakeCheckHashPath(root, "libs", "global", node.Name)
		return label, path, hashPath, nil
	case NodeProject:
		path := filepath.Join(root, "repos", "projects", node.Name)
		label := fmt.Sprintf("project %s", node.Name)
		hashPath := hash.MakeCheckHashPath(root, "projects", node.Name)
		return label, path, hashPath, nil
	default:
		return "", "", "", fmt.Errorf("unsupported dependency node")
	}
}

func resolveGlobalDeps(deps []string, globals map[string]string) []string {
	resolved := make([]string, 0, len(deps))
	for _, dep := range deps {
		if key, ok := globals[dep]; ok {
			resolved = append(resolved, key)
		}
	}
	return resolved
}

func resolveAppDeps(appName string, deps []string, appLibs map[string]map[string]string, globals map[string]string, projects map[string]string) []string {
	resolved := make([]string, 0, len(deps))
	appEntries := appLibs[appName]
	for _, dep := range deps {
		if appEntries != nil {
			if key, ok := appEntries[dep]; ok {
				resolved = append(resolved, key)
				continue
			}
		}
		if key, ok := globals[dep]; ok {
			resolved = append(resolved, key)
			continue
		}
		if key, ok := projects[dep]; ok {
			resolved = append(resolved, key)
		}
	}
	return resolved
}

func findGlobalLibraryByName(config workspace.WorkspaceConfig, name string) (workspace.WorkspaceLibrary, bool, error) {
	var match *workspace.WorkspaceLibrary
	for i, lib := range config.Libraries {
		if lib.Name != name {
			continue
		}
		if match != nil {
			return workspace.WorkspaceLibrary{}, false, fmt.Errorf("global library name is ambiguous: %s", name)
		}
		match = &config.Libraries[i]
	}
	if match == nil {
		return workspace.WorkspaceLibrary{}, false, nil
	}
	return *match, true, nil
}

func findProjectByName(config workspace.WorkspaceConfig, name string) (workspace.WorkspaceProject, bool, error) {
	var match *workspace.WorkspaceProject
	for i, project := range config.Projects {
		if project.Name != name {
			continue
		}
		if match != nil {
			return workspace.WorkspaceProject{}, false, fmt.Errorf("project name is ambiguous: %s", name)
		}
		match = &config.Projects[i]
	}
	if match == nil {
		return workspace.WorkspaceProject{}, false, nil
	}
	return *match, true, nil
}

func findAppLibraryByName(config workspace.WorkspaceConfig, appName string, name string) (workspace.WorkspaceLibrary, bool) {
	for _, app := range config.Apps {
		if app.Name != appName {
			continue
		}
		for _, lib := range app.Libraries {
			if lib.Name == name {
				return lib, true
			}
		}
	}
	return workspace.WorkspaceLibrary{}, false
}

func makeNode(kind NodeKind, app string, name string, deps ...string) Node {
	return Node{
		Kind:     kind,
		App:      app,
		Name:     name,
		Deps:     deps,
	}
}
