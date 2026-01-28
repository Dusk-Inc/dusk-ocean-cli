package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/deps"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/prompts"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/tree"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/workspace"
	"github.com/manifoldco/promptui"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

type targetKind = workspace.TargetKind
type dependencyKind string

const (
	targetService   targetKind = workspace.TargetService
	targetAppLib    targetKind = workspace.TargetAppLib
	targetGlobalLib targetKind = workspace.TargetGlobalLib
	targetProject   targetKind = workspace.TargetProject

	dependencyGlobalLib dependencyKind = "global-lib"
	dependencyAppLib    dependencyKind = "app-lib"
	dependencyProject   dependencyKind = "project"
)

type installTarget = workspace.Target

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
	installCmd, err := workspace.ReadRepoCommand(afero.NewOsFs(), dependency.path, "install")
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
	execCmd.Dir = target.Path
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
	target, err := workspace.ResolveTargetFromCwd(afero.NewOsFs(), root, cwd)
	if err != nil {
		return err
	}
	config, err := workspace.ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	if err := workspace.ValidateTargetRegistration(target, config); err != nil {
		return err
	}
	dependency, err := resolveDependency(root, target, dependencyName, config)
	if err != nil {
		return err
	}
	if err := validateInstallFlow(target, dependency); err != nil {
		return err
	}
	installCmd, err := workspace.ReadRepoCommand(afero.NewOsFs(), dependency.path, "install")
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
	execCmd.Dir = target.Path
	execCmd.Stdout = cmd.OutOrStdout()
	execCmd.Stderr = cmd.ErrOrStderr()
	if err := execCmd.Run(); err != nil {
		return err
	}
	return workspace.WriteWorkspaceConfig(fs, updatedConfig)
}

func resolveDependency(root string, target installTarget, name string, config workspace.WorkspaceConfig) (installDependency, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return installDependency{}, fmt.Errorf("dependency name is required")
	}

	var matches []installDependency

	if target.Kind == targetAppLib || target.Kind == targetService {
		if target.App != "" {
			appLibPath := filepath.Join(root, "repos", "apps", target.App, "libs", name)
			if tree.DirExists(afero.NewOsFs(), appLibPath) {
				appIndex := workspace.FindAppIndex(config, target.App)
				if appIndex == -1 {
					return installDependency{}, fmt.Errorf("app not registered in workspace: %s", target.App)
				}
				libIndex := workspace.FindAppLibraryIndex(config.Apps[appIndex], name)
				if libIndex == -1 {
					return installDependency{}, fmt.Errorf("library not registered in workspace: %s", name)
				}
				matches = append(matches, installDependency{
					kind:     dependencyAppLib,
					app:      target.App,
					name:     name,
					path:     appLibPath,
				})
			}
		}
	}

	if lib, ok, err := workspace.FindGlobalLibraryByName(config, name); err != nil {
		return installDependency{}, err
	} else if ok {
		depPath := filepath.Join(root, "repos", "libs", lib.Name)
		matches = append(matches, installDependency{
			kind:     dependencyGlobalLib,
			name:     name,
			path:     depPath,
		})
	}

	if project, ok, err := workspace.FindProjectByName(config, name); err != nil {
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
		if target.Kind == targetService || target.Kind == targetAppLib {
			servicePath := filepath.Join(root, "repos", "apps", target.App, "services", name)
			if tree.DirExists(afero.NewOsFs(), servicePath) {
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
	allowedDeps, ok := allowedInstallDependencies[target.Kind]
	if !ok {
		return fmt.Errorf("unsupported install target")
	}
	if !allowedDeps[dependency.kind] {
		switch target.Kind {
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
	if dependency.kind == dependencyAppLib && target.App != dependency.app {
		return fmt.Errorf("app-scoped libraries must stay within the same app")
	}
	return nil
}

func promptForDependency(config workspace.WorkspaceConfig, root string) (installDependency, error) {
	options := []string{}
	if len(config.Libraries) > 0 {
		options = append(options, "global library")
	}
	appNames := workspace.AppNamesWithLibraries(config)
	if len(appNames) > 0 {
		options = append(options, "app library")
	}
	if len(options) == 0 {
		return installDependency{}, fmt.Errorf("no libraries available to install")
	}
	scope := options[0]
	if len(options) > 1 {
		selected, err := prompts.SelectFromList("Select payload source", options)
		if err != nil {
			return installDependency{}, err
		}
		scope = selected
	}
	if scope == "app library" {
		appName, err := prompts.SelectFromList("Select app", appNames)
		if err != nil {
			return installDependency{}, err
		}
		appLibs := workspace.AppLibraryNames(config, appName)
		if len(appLibs) == 0 {
			return installDependency{}, fmt.Errorf("no libraries found for app: %s", appName)
		}
		libName, err := prompts.SelectFromList("Select library", appLibs)
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
	globalLibs := workspace.GlobalLibraryNames(config)
	libName, err := prompts.SelectFromList("Select library", globalLibs)
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
	appLibApps := workspace.AppNamesWithLibraries(config)
	if len(appLibApps) > 0 {
		options = append(options, "app library")
	}
	serviceApps := workspace.AppNamesWithServices(config)
	if len(serviceApps) > 0 {
		options = append(options, "service")
	}
	if len(options) == 0 {
		return installTarget{}, fmt.Errorf("no targets available for install")
	}
	targetKindLabel := options[0]
	if len(options) > 1 {
		selected, err := prompts.SelectFromList("Select target type", options)
		if err != nil {
			return installTarget{}, err
		}
		targetKindLabel = selected
	}
	switch targetKindLabel {
	case "service":
		appName, err := prompts.SelectFromList("Select app", serviceApps)
		if err != nil {
			return installTarget{}, err
		}
		serviceNames := workspace.ServiceNames(config, appName)
		if len(serviceNames) == 0 {
			return installTarget{}, fmt.Errorf("no services found for app: %s", appName)
		}
		name, err := prompts.SelectFromList("Select service", serviceNames)
		if err != nil {
			return installTarget{}, err
		}
		return installTarget{
			Kind: targetService,
			App:  appName,
			Name: name,
			Path: filepath.Join(root, "repos", "apps", appName, "services", name),
		}, nil
	case "app library":
		appName, err := prompts.SelectFromList("Select app", appLibApps)
		if err != nil {
			return installTarget{}, err
		}
		libNames := workspace.AppLibraryNames(config, appName)
		if len(libNames) == 0 {
			return installTarget{}, fmt.Errorf("no libraries found for app: %s", appName)
		}
		name, err := prompts.SelectFromList("Select library", libNames)
		if err != nil {
			return installTarget{}, err
		}
		return installTarget{
			Kind: targetAppLib,
			App:  appName,
			Name: name,
			Path: filepath.Join(root, "repos", "apps", appName, "libs", name),
		}, nil
	case "project":
		projectNames := workspace.ProjectNames(config)
		name, err := prompts.SelectFromList("Select project", projectNames)
		if err != nil {
			return installTarget{}, err
		}
		return installTarget{
			Kind: targetProject,
			Name: name,
			Path: filepath.Join(root, "repos", "projects", name),
		}, nil
	case "global library":
		globalLibs := workspace.GlobalLibraryNames(config)
		name, err := prompts.SelectFromList("Select library", globalLibs)
		if err != nil {
			return installTarget{}, err
		}
		return installTarget{
			Kind: targetGlobalLib,
			Name: name,
			Path: filepath.Join(root, "repos", "libs", name),
		}, nil
	default:
		return installTarget{}, fmt.Errorf("unsupported target type")
	}
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
	switch target.Kind {
	case targetService:
		return fmt.Sprintf("service %s/%s", target.App, target.Name)
	case targetAppLib:
		return fmt.Sprintf("app library %s/%s", target.App, target.Name)
	case targetGlobalLib:
		return fmt.Sprintf("global library %s", target.Name)
	case targetProject:
		return fmt.Sprintf("project %s", target.Name)
	default:
		return target.Name
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

func registerDependency(config workspace.WorkspaceConfig, target installTarget, dependency installDependency) (workspace.WorkspaceConfig, error) {
	targetKey := installTargetKey(target)
	depKey := installDependencyKey(dependency)
	if targetKey == depKey {
		return workspace.WorkspaceConfig{}, fmt.Errorf("dependency cannot reference itself")
	}

	switch target.Kind {
	case targetService:
		appIndex := workspace.FindAppIndex(config, target.App)
		if appIndex == -1 {
			return workspace.WorkspaceConfig{}, fmt.Errorf("app not registered in workspace: %s", target.App)
		}
		serviceIndex := workspace.FindServiceIndex(config.Apps[appIndex], target.Name)
		if serviceIndex == -1 {
			return workspace.WorkspaceConfig{}, fmt.Errorf("service not registered in workspace: %s", target.Name)
		}
		depsList := config.Apps[appIndex].Services[serviceIndex].Deps
		if containsDep(depsList, dependency.name, depSourceForInstall(dependency)) {
			return workspace.WorkspaceConfig{}, fmt.Errorf("dependency already registered: %s", dependency.name)
		}
		config.Apps[appIndex].Services[serviceIndex].Deps = append(depsList, makeInstallDep(dependency))
	case targetAppLib:
		appIndex := workspace.FindAppIndex(config, target.App)
		if appIndex == -1 {
			return workspace.WorkspaceConfig{}, fmt.Errorf("app not registered in workspace: %s", target.App)
		}
		libIndex := workspace.FindAppLibraryIndex(config.Apps[appIndex], target.Name)
		if libIndex == -1 {
			return workspace.WorkspaceConfig{}, fmt.Errorf("library not registered in workspace: %s", target.Name)
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
		libIndex := workspace.FindGlobalLibraryIndex(config, target.Name)
		if libIndex == -1 {
			return workspace.WorkspaceConfig{}, fmt.Errorf("library not registered in workspace: %s", target.Name)
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
		projectIndex := workspace.FindProjectIndex(config, target.Name)
		if projectIndex == -1 {
			return workspace.WorkspaceConfig{}, fmt.Errorf("project not registered in workspace: %s", target.Name)
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
	if target.Kind == targetService {
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
	switch target.Kind {
	case targetService:
		return deps.ServiceKey(target.App, target.Name)
	case targetAppLib:
		return deps.AppLibKey(target.App, target.Name)
	case targetGlobalLib:
		return deps.GlobalLibKey(target.Name)
	case targetProject:
		return deps.ProjectKey(target.Name)
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
