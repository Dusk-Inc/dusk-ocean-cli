package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/apps"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/deps"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/libraries"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/projects"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/services"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/tree"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/workspace"
	"github.com/manifoldco/promptui"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

type targetKind string
type dependencyKind string

const (
	targetService   targetKind = "service"
	targetAppLib    targetKind = "app-lib"
	targetGlobalLib targetKind = "global-lib"
	targetProject   targetKind = "project"

	dependencyGlobalLib dependencyKind = "global-lib"
	dependencyAppLib    dependencyKind = "app-lib"
	dependencyProject   dependencyKind = "project"
)

type installTarget struct {
	kind     targetKind
	app      string
	name     string
	path     string
}

type installDependency struct {
	kind       dependencyKind
	app        string
	name       string
	path       string
	installCmd string
}

var allowedInstallDependencies = map[targetKind]map[dependencyKind]bool{
	targetGlobalLib: {
		dependencyGlobalLib: true,
	},
	targetProject: {
		dependencyGlobalLib: true,
	},
	targetAppLib: {
		dependencyAppLib:    true,
		dependencyGlobalLib: true,
		dependencyProject:   true,
	},
	targetService: {
		dependencyAppLib:    true,
		dependencyGlobalLib: true,
		dependencyProject:   true,
	},
}

func RunInstallPrompt(cmd *cobra.Command, fs afero.Fs) error {
	root, err := ensureWorkspaceRoot(fs)
	if err != nil {
		return err
	}
	config, err := workspace.ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	dependency, err := promptForDependency(config, root)
	if err != nil {
		return err
	}
	target, err := promptForTarget(config, root)
	if err != nil {
		return err
	}
	if err := validateInstallFlow(target, dependency); err != nil {
		return err
	}
	targetKey := installTargetKey(target)
	depKey := installDependencyKey(dependency)
	if targetKey != "" && targetKey == depKey {
		return fmt.Errorf("payload and target cannot be the same")
	}
	confirmPrompt := promptui.Prompt{
		Label:     fmt.Sprintf("Install %s into %s", formatDependencyLabel(dependency), formatTargetLabel(target)),
		IsConfirm: true,
	}
	confirm, err := confirmPrompt.Run()
	if err != nil {
		return err
	}
	if strings.ToLower(confirm) != "y" {
		return fmt.Errorf("aborted")
	}
	installCmd, err := readInstallCommand(dependency.path)
	if err != nil {
		return err
	}
	if strings.TrimSpace(installCmd) == "" {
		return fmt.Errorf("install command missing for %s", dependency.name)
	}
	dependency.installCmd = installCmd
	updatedConfig, err := registerDependency(config, target, dependency)
	if err != nil {
		return err
	}
	execCmd := exec.Command("bash", "-lc", dependency.installCmd)
	execCmd.Dir = target.path
	execCmd.Stdout = cmd.OutOrStdout()
	execCmd.Stderr = cmd.ErrOrStderr()
	if err := execCmd.Run(); err != nil {
		return err
	}
	return workspace.WriteWorkspaceConfig(fs, updatedConfig)
}

func RunInstallFromCwd(cmd *cobra.Command, fs afero.Fs, dependencyName string) error {
	root, err := tree.GetRoot()
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	target, err := resolveTargetFromCwd(root, cwd)
	if err != nil {
		return err
	}
	config, err := workspace.ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	if err := validateTargetRegistration(target, config); err != nil {
		return err
	}
	dependency, err := resolveDependency(root, target, dependencyName, config)
	if err != nil {
		return err
	}
	if err := validateInstallFlow(target, dependency); err != nil {
		return err
	}
	installCmd, err := readInstallCommand(dependency.path)
	if err != nil {
		return err
	}
	if strings.TrimSpace(installCmd) == "" {
		return fmt.Errorf("install command missing for %s", dependency.name)
	}
	dependency.installCmd = installCmd
	updatedConfig, err := registerDependency(config, target, dependency)
	if err != nil {
		return err
	}
	execCmd := exec.Command("bash", "-lc", dependency.installCmd)
	execCmd.Dir = target.path
	execCmd.Stdout = cmd.OutOrStdout()
	execCmd.Stderr = cmd.ErrOrStderr()
	if err := execCmd.Run(); err != nil {
		return err
	}
	return workspace.WriteWorkspaceConfig(fs, updatedConfig)
}

func resolveTargetFromCwd(root string, cwd string) (installTarget, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return installTarget{}, err
	}
	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return installTarget{}, err
	}
	rel, err := filepath.Rel(absRoot, absCwd)
	if err != nil {
		return installTarget{}, err
	}
	parts := strings.Split(filepath.Clean(rel), string(filepath.Separator))
	if len(parts) < 4 || parts[0] != "repos" {
		return installTarget{}, fmt.Errorf("current directory is not a valid install target")
	}

	switch parts[1] {
	case "apps":
		if len(parts) >= 5 && parts[3] == "services" {
			targetPath := filepath.Join(absRoot, "repos", "apps", parts[2], "services", parts[4])
			if !dirExists(targetPath) {
				return installTarget{}, fmt.Errorf("service path does not exist: %s", targetPath)
			}
			return installTarget{
				kind: targetService,
				app:  parts[2],
				name: parts[4],
				path: targetPath,
			}, nil
		}
		if len(parts) >= 5 && parts[3] == "libs" {
			targetPath := filepath.Join(absRoot, "repos", "apps", parts[2], "libs", parts[4])
			if !dirExists(targetPath) {
				return installTarget{}, fmt.Errorf("library path does not exist: %s", targetPath)
			}
			return installTarget{
				kind: targetAppLib,
				app:  parts[2],
				name: parts[4],
				path: targetPath,
			}, nil
		}
	case "libs":
		if len(parts) >= 3 {
			targetPath := filepath.Join(absRoot, "repos", "libs", parts[2])
			if !dirExists(targetPath) {
				return installTarget{}, fmt.Errorf("library path does not exist: %s", targetPath)
			}
			return installTarget{
				kind:     targetGlobalLib,
				name:     parts[2],
				path:     targetPath,
			}, nil
		}
	case "projects":
		if len(parts) >= 3 {
			targetPath := filepath.Join(absRoot, "repos", "projects", parts[2])
			if !dirExists(targetPath) {
				return installTarget{}, fmt.Errorf("project path does not exist: %s", targetPath)
			}
			return installTarget{
				kind:     targetProject,
				name:     parts[2],
				path:     targetPath,
			}, nil
		}
	}

	return installTarget{}, fmt.Errorf("current directory is not a supported install target")
}

func validateTargetRegistration(target installTarget, config workspace.WorkspaceConfig) error {
	switch target.kind {
	case targetService:
		appIndex := apps.FindAppIndex(config, target.app)
		if appIndex == -1 {
			return fmt.Errorf("app not registered in workspace: %s", target.app)
		}
		if services.FindServiceIndex(config.Apps[appIndex], target.name) == -1 {
			return fmt.Errorf("service not registered in workspace: %s", target.name)
		}
	case targetAppLib:
		appIndex := apps.FindAppIndex(config, target.app)
		if appIndex == -1 {
			return fmt.Errorf("app not registered in workspace: %s", target.app)
		}
		if libraries.FindAppLibraryIndex(config.Apps[appIndex], target.name) == -1 {
			return fmt.Errorf("library not registered in workspace: %s", target.name)
		}
	case targetGlobalLib:
		libIndex := libraries.FindGlobalLibraryIndex(config, target.name)
		if libIndex == -1 {
			return fmt.Errorf("library not registered in workspace: %s", target.name)
		}
	case targetProject:
		projectIndex := projects.FindProjectIndex(config, target.name)
		if projectIndex == -1 {
			return fmt.Errorf("project not registered in workspace: %s", target.name)
		}
	default:
		return fmt.Errorf("unsupported install target")
	}
	return nil
}

func resolveDependency(root string, target installTarget, name string, config workspace.WorkspaceConfig) (installDependency, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return installDependency{}, fmt.Errorf("dependency name is required")
	}

	var matches []installDependency

	if target.kind == targetAppLib || target.kind == targetService {
		if target.app != "" {
			appLibPath := filepath.Join(root, "repos", "apps", target.app, "libs", name)
			if dirExists(appLibPath) {
				appIndex := apps.FindAppIndex(config, target.app)
				if appIndex == -1 {
					return installDependency{}, fmt.Errorf("app not registered in workspace: %s", target.app)
				}
				libIndex := libraries.FindAppLibraryIndex(config.Apps[appIndex], name)
				if libIndex == -1 {
					return installDependency{}, fmt.Errorf("library not registered in workspace: %s", name)
				}
				matches = append(matches, installDependency{
					kind:     dependencyAppLib,
					app:      target.app,
					name:     name,
					path:     appLibPath,
				})
			}
		}
	}

	if lib, ok, err := libraries.FindGlobalLibraryByName(config, name); err != nil {
		return installDependency{}, err
	} else if ok {
		depPath := filepath.Join(root, "repos", "libs", lib.Name)
		matches = append(matches, installDependency{
			kind:     dependencyGlobalLib,
			name:     name,
			path:     depPath,
		})
	}

	if project, ok, err := projects.FindProjectByName(config, name); err != nil {
		return installDependency{}, err
	} else if ok {
		depPath := filepath.Join(root, "repos", "projects", project.Name)
		matches = append(matches, installDependency{
			kind:     dependencyProject,
			name:     name,
			path:     depPath,
		})
	}

	if len(matches) == 0 {
		if target.kind == targetService || target.kind == targetAppLib {
			servicePath := filepath.Join(root, "repos", "apps", target.app, "services", name)
			if dirExists(servicePath) {
				return installDependency{}, fmt.Errorf("services cannot be dependencies")
			}
		}
		return installDependency{}, fmt.Errorf("dependency not found: %s", name)
	}
	if len(matches) > 1 {
		return installDependency{}, fmt.Errorf("dependency name is ambiguous: %s", name)
	}
	return matches[0], nil
}

func validateInstallFlow(target installTarget, dependency installDependency) error {
	allowedDeps, ok := allowedInstallDependencies[target.kind]
	if !ok {
		return fmt.Errorf("unsupported install target")
	}
	if !allowedDeps[dependency.kind] {
		switch target.kind {
		case targetGlobalLib:
			return fmt.Errorf("invalid dependency for global library")
		case targetProject:
			return fmt.Errorf("invalid dependency for project")
		case targetAppLib:
			return fmt.Errorf("invalid dependency for app library")
		case targetService:
			return fmt.Errorf("invalid dependency for service")
		default:
			return fmt.Errorf("unsupported install target")
		}
	}
	if dependency.kind == dependencyAppLib && target.app != dependency.app {
		return fmt.Errorf("app-scoped libraries must stay within the same app")
	}
	return nil
}

func promptForDependency(config workspace.WorkspaceConfig, root string) (installDependency, error) {
	options := []string{}
	if len(config.Libraries) > 0 {
		options = append(options, "global library")
	}
	appNames := appNamesWithLibraries(config)
	if len(appNames) > 0 {
		options = append(options, "app library")
	}
	if len(options) == 0 {
		return installDependency{}, fmt.Errorf("no libraries available to install")
	}
	scope := options[0]
	if len(options) > 1 {
		selected, err := selectOption("Select payload source", options)
		if err != nil {
			return installDependency{}, err
		}
		scope = selected
	}
	if scope == "app library" {
		appName, err := selectOption("Select app", appNames)
		if err != nil {
			return installDependency{}, err
		}
		appLibs := appLibraryNames(config, appName)
		if len(appLibs) == 0 {
			return installDependency{}, fmt.Errorf("no libraries found for app: %s", appName)
		}
		libName, err := selectOption("Select library", appLibs)
		if err != nil {
			return installDependency{}, err
		}
		return installDependency{
			kind: dependencyAppLib,
			app:  appName,
			name: libName,
			path: filepath.Join(root, "repos", "apps", appName, "libs", libName),
		}, nil
	}
	globalLibs := globalLibraryNames(config)
	libName, err := selectOption("Select library", globalLibs)
	if err != nil {
		return installDependency{}, err
	}
	return installDependency{
		kind: dependencyGlobalLib,
		name: libName,
		path: filepath.Join(root, "repos", "libs", libName),
	}, nil
}

func promptForTarget(config workspace.WorkspaceConfig, root string) (installTarget, error) {
	options := []string{}
	if len(config.Libraries) > 0 {
		options = append(options, "global library")
	}
	if len(config.Projects) > 0 {
		options = append(options, "project")
	}
	appLibApps := appNamesWithLibraries(config)
	if len(appLibApps) > 0 {
		options = append(options, "app library")
	}
	serviceApps := appNamesWithServices(config)
	if len(serviceApps) > 0 {
		options = append(options, "service")
	}
	if len(options) == 0 {
		return installTarget{}, fmt.Errorf("no targets available for install")
	}
	targetKindLabel := options[0]
	if len(options) > 1 {
		selected, err := selectOption("Select target type", options)
		if err != nil {
			return installTarget{}, err
		}
		targetKindLabel = selected
	}
	switch targetKindLabel {
	case "service":
		appName, err := selectOption("Select app", serviceApps)
		if err != nil {
			return installTarget{}, err
		}
		serviceNames := serviceNames(config, appName)
		if len(serviceNames) == 0 {
			return installTarget{}, fmt.Errorf("no services found for app: %s", appName)
		}
		name, err := selectOption("Select service", serviceNames)
		if err != nil {
			return installTarget{}, err
		}
		return installTarget{
			kind: targetService,
			app:  appName,
			name: name,
			path: filepath.Join(root, "repos", "apps", appName, "services", name),
		}, nil
	case "app library":
		appName, err := selectOption("Select app", appLibApps)
		if err != nil {
			return installTarget{}, err
		}
		libNames := appLibraryNames(config, appName)
		if len(libNames) == 0 {
			return installTarget{}, fmt.Errorf("no libraries found for app: %s", appName)
		}
		name, err := selectOption("Select library", libNames)
		if err != nil {
			return installTarget{}, err
		}
		return installTarget{
			kind: targetAppLib,
			app:  appName,
			name: name,
			path: filepath.Join(root, "repos", "apps", appName, "libs", name),
		}, nil
	case "project":
		projectNames := projectNames(config)
		name, err := selectOption("Select project", projectNames)
		if err != nil {
			return installTarget{}, err
		}
		return installTarget{
			kind: targetProject,
			name: name,
			path: filepath.Join(root, "repos", "projects", name),
		}, nil
	case "global library":
		globalLibs := globalLibraryNames(config)
		name, err := selectOption("Select library", globalLibs)
		if err != nil {
			return installTarget{}, err
		}
		return installTarget{
			kind: targetGlobalLib,
			name: name,
			path: filepath.Join(root, "repos", "libs", name),
		}, nil
	default:
		return installTarget{}, fmt.Errorf("unsupported target type")
	}
}

func selectOption(label string, items []string) (string, error) {
	if len(items) == 0 {
		return "", fmt.Errorf("no options available")
	}
	prompt := promptui.Select{
		Label: label,
		Items: items,
	}
	_, selected, err := prompt.Run()
	if err != nil {
		return "", err
	}
	return selected, nil
}

func appNamesWithLibraries(config workspace.WorkspaceConfig) []string {
	names := []string{}
	for _, app := range config.Apps {
		if len(app.Libraries) == 0 {
			continue
		}
		names = append(names, app.Name)
	}
	return names
}

func appNamesWithServices(config workspace.WorkspaceConfig) []string {
	names := []string{}
	for _, app := range config.Apps {
		if len(app.Services) == 0 {
			continue
		}
		names = append(names, app.Name)
	}
	return names
}

func appLibraryNames(config workspace.WorkspaceConfig, appName string) []string {
	for _, app := range config.Apps {
		if app.Name != appName {
			continue
		}
		names := make([]string, 0, len(app.Libraries))
		for _, lib := range app.Libraries {
			names = append(names, lib.Name)
		}
		return names
	}
	return nil
}

func serviceNames(config workspace.WorkspaceConfig, appName string) []string {
	for _, app := range config.Apps {
		if app.Name != appName {
			continue
		}
		names := make([]string, 0, len(app.Services))
		for _, service := range app.Services {
			names = append(names, service.Name)
		}
		return names
	}
	return nil
}

func globalLibraryNames(config workspace.WorkspaceConfig) []string {
	names := make([]string, 0, len(config.Libraries))
	for _, lib := range config.Libraries {
		names = append(names, lib.Name)
	}
	return names
}

func projectNames(config workspace.WorkspaceConfig) []string {
	names := make([]string, 0, len(config.Projects))
	for _, project := range config.Projects {
		names = append(names, project.Name)
	}
	return names
}

func formatDependencyLabel(dep installDependency) string {
	switch dep.kind {
	case dependencyAppLib:
		return fmt.Sprintf("app library %s/%s", dep.app, dep.name)
	case dependencyGlobalLib:
		return fmt.Sprintf("global library %s", dep.name)
	case dependencyProject:
		return fmt.Sprintf("project %s", dep.name)
	default:
		return dep.name
	}
}

func formatTargetLabel(target installTarget) string {
	switch target.kind {
	case targetService:
		return fmt.Sprintf("service %s/%s", target.app, target.name)
	case targetAppLib:
		return fmt.Sprintf("app library %s/%s", target.app, target.name)
	case targetGlobalLib:
		return fmt.Sprintf("global library %s", target.name)
	case targetProject:
		return fmt.Sprintf("project %s", target.name)
	default:
		return target.name
	}
}

func ensureWorkspaceRoot(fs afero.Fs) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	root, err := tree.GetRoot()
	if err != nil {
		return "", err
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return "", err
	}
	if absRoot != absCwd {
		return "", fmt.Errorf("install must be run from the workspace root (%s)", absRoot)
	}
	if _, err := fs.Stat("ocean.workspace.json"); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("workspace config not found: ocean.workspace.json")
		}
		return "", err
	}
	return absRoot, nil
}

func readInstallCommand(targetPath string) (string, error) {
	config, err := workspace.ReadRepoConfig(afero.NewOsFs(), targetPath)
	if err != nil {
		return "", err
	}
	return workspace.RepoCommand(config, "install")
}

func registerDependency(config workspace.WorkspaceConfig, target installTarget, dependency installDependency) (workspace.WorkspaceConfig, error) {
	targetKey := installTargetKey(target)
	depKey := installDependencyKey(dependency)
	if targetKey == depKey {
		return workspace.WorkspaceConfig{}, fmt.Errorf("dependency cannot reference itself")
	}

	switch target.kind {
	case targetService:
		appIndex := apps.FindAppIndex(config, target.app)
		if appIndex == -1 {
			return workspace.WorkspaceConfig{}, fmt.Errorf("app not registered in workspace: %s", target.app)
		}
		serviceIndex := services.FindServiceIndex(config.Apps[appIndex], target.name)
		if serviceIndex == -1 {
			return workspace.WorkspaceConfig{}, fmt.Errorf("service not registered in workspace: %s", target.name)
		}
		depsList := config.Apps[appIndex].Services[serviceIndex].Deps
		if containsDep(depsList, dependency.name, depSourceForInstall(dependency)) {
			return workspace.WorkspaceConfig{}, fmt.Errorf("dependency already registered: %s", dependency.name)
		}
		config.Apps[appIndex].Services[serviceIndex].Deps = append(depsList, makeInstallDep(dependency))
	case targetAppLib:
		appIndex := apps.FindAppIndex(config, target.app)
		if appIndex == -1 {
			return workspace.WorkspaceConfig{}, fmt.Errorf("app not registered in workspace: %s", target.app)
		}
		libIndex := libraries.FindAppLibraryIndex(config.Apps[appIndex], target.name)
		if libIndex == -1 {
			return workspace.WorkspaceConfig{}, fmt.Errorf("library not registered in workspace: %s", target.name)
		}
		depsList := config.Apps[appIndex].Libraries[libIndex].Deps
		if containsDep(depsList, dependency.name, depSourceForInstall(dependency)) {
			return workspace.WorkspaceConfig{}, fmt.Errorf("dependency already registered: %s", dependency.name)
		}
		if err := ensureNoCycles(config, target, dependency); err != nil {
			return workspace.WorkspaceConfig{}, err
		}
		config.Apps[appIndex].Libraries[libIndex].Deps = append(depsList, makeInstallDep(dependency))
	case targetGlobalLib:
		libIndex := libraries.FindGlobalLibraryIndex(config, target.name)
		if libIndex == -1 {
			return workspace.WorkspaceConfig{}, fmt.Errorf("library not registered in workspace: %s", target.name)
		}
		depsList := config.Libraries[libIndex].Deps
		if containsDep(depsList, dependency.name, depSourceForInstall(dependency)) {
			return workspace.WorkspaceConfig{}, fmt.Errorf("dependency already registered: %s", dependency.name)
		}
		if err := ensureNoCycles(config, target, dependency); err != nil {
			return workspace.WorkspaceConfig{}, err
		}
		config.Libraries[libIndex].Deps = append(depsList, makeInstallDep(dependency))
	case targetProject:
		projectIndex := projects.FindProjectIndex(config, target.name)
		if projectIndex == -1 {
			return workspace.WorkspaceConfig{}, fmt.Errorf("project not registered in workspace: %s", target.name)
		}
		depsList := config.Projects[projectIndex].Deps
		if containsDep(depsList, dependency.name, depSourceForInstall(dependency)) {
			return workspace.WorkspaceConfig{}, fmt.Errorf("dependency already registered: %s", dependency.name)
		}
		if err := ensureNoCycles(config, target, dependency); err != nil {
			return workspace.WorkspaceConfig{}, err
		}
		config.Projects[projectIndex].Deps = append(depsList, makeInstallDep(dependency))
	default:
		return workspace.WorkspaceConfig{}, fmt.Errorf("unsupported install target")
	}

	return config, nil
}

func ensureNoCycles(config workspace.WorkspaceConfig, target installTarget, dependency installDependency) error {
	if target.kind == targetService {
		return nil
	}
	graph, err := deps.BuildDependencyGraph(config)
	if err != nil {
		return err
	}
	targetKey := installTargetKey(target)
	depKey := installDependencyKey(dependency)
	if targetKey == "" || depKey == "" {
		return nil
	}
	if _, ok := graph[targetKey]; !ok {
		graph[targetKey] = []string{}
	}
	if _, ok := graph[depKey]; !ok {
		graph[depKey] = []string{}
	}
	if deps.HasPath(graph, depKey, targetKey) {
		return fmt.Errorf("dependency would create a cycle")
	}
	return nil
}

func installTargetKey(target installTarget) string {
	switch target.kind {
	case targetService:
		return deps.ServiceKey(target.app, target.name)
	case targetAppLib:
		return deps.AppLibKey(target.app, target.name)
	case targetGlobalLib:
		return deps.GlobalLibKey(target.name)
	case targetProject:
		return deps.ProjectKey(target.name)
	default:
		return ""
	}
}

func installDependencyKey(dep installDependency) string {
	switch dep.kind {
	case dependencyAppLib:
		return deps.AppLibKey(dep.app, dep.name)
	case dependencyGlobalLib:
		return deps.GlobalLibKey(dep.name)
	case dependencyProject:
		return deps.ProjectKey(dep.name)
	default:
		return ""
	}
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func containsDep(values []workspace.WorkspaceDep, lib string, from string) bool {
	for _, value := range values {
		if value.Lib == lib && value.From == from {
			return true
		}
	}
	return false
}

func makeInstallDep(dep installDependency) workspace.WorkspaceDep {
	return workspace.WorkspaceDep{
		Lib:  dep.name,
		From: depSourceForInstall(dep),
	}
}

func depSourceForInstall(dep installDependency) string {
	switch dep.kind {
	case dependencyAppLib:
		return dep.app
	case dependencyGlobalLib:
		return "global"
	case dependencyProject:
		return "project"
	default:
		return ""
	}
}
