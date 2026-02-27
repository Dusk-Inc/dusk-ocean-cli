package functions

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

type targetKind = TargetKind
type dependencyKind string

const (
	targetService   targetKind = TargetService
	targetAppLib    targetKind = TargetAppLib
	targetGlobalLib targetKind = TargetGlobalLib
	targetProject   targetKind = TargetProject
	targetTest      targetKind = TargetTest

	dependencyGlobalLib dependencyKind = "global-lib"
	dependencyAppLib    dependencyKind = "app-lib"
	dependencyProject   dependencyKind = "project"
)

type installTarget = Target

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
	targetTest: {
		dependencyAppLib:    true,
		dependencyGlobalLib: true,
		dependencyProject:   true,
	},
}

func RunInstallPrompt(cmd *cobra.Command, fs afero.Fs) error {
	root, err := EnsureWorkspaceRoot(fs)
	if err != nil {
		return err
	}
	config, err := ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	dependency, err := promptForDependency(config, root)
	if err != nil {
		return err
	}
	target, err := PromptForTarget(config, root)
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
		Label:     fmt.Sprintf("Install %s into %s", formatDependencyLabel(dependency), FormatTargetLabel(target)),
		IsConfirm: true,
	}
	confirm, err := confirmPrompt.Run()
	if err != nil {
		return err
	}
	if strings.ToLower(confirm) != "y" {
		return fmt.Errorf("aborted")
	}
	installCmd, err := ReadRepoCommand(afero.NewOsFs(), dependency.path, "add")
	if err != nil {
		return err
	}
	if strings.TrimSpace(installCmd) == "" {
		return fmt.Errorf("add command missing for %s", dependency.name)
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
	return WriteWorkspaceConfig(fs, updatedConfig)
}

func RunInstallFromCwd(cmd *cobra.Command, fs afero.Fs, dependencyName string) error {
	root, err := GetRoot()
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	target, err := ResolveTarget(afero.NewOsFs(), root, cwd)
	if err != nil {
		return err
	}
	config, err := ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	if err := ValidateTargetRegistration(target, config); err != nil {
		return err
	}
	dependency, err := resolveDependency(root, target, dependencyName, config)
	if err != nil {
		return err
	}
	if err := validateInstallFlow(target, dependency); err != nil {
		return err
	}
	installCmd, err := ReadRepoCommand(afero.NewOsFs(), dependency.path, "add")
	if err != nil {
		return err
	}
	if strings.TrimSpace(installCmd) == "" {
		return fmt.Errorf("add command missing for %s", dependency.name)
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
	return WriteWorkspaceConfig(fs, updatedConfig)
}

func resolveDependency(root string, target installTarget, name string, config WorkspaceConfig) (installDependency, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return installDependency{}, fmt.Errorf("dependency name is required")
	}

	var matches []installDependency

	if target.Kind == targetAppLib || target.Kind == targetService || target.Kind == targetTest {
		if target.App != "" {
			appLibPath := filepath.Join(root, "repos", "apps", target.App, "libs", name)
			if DirExists(afero.NewOsFs(), appLibPath) {
				appIndex := FindAppIndex(config, target.App)
				if appIndex == -1 {
					return installDependency{}, fmt.Errorf("app not registered in workspace: %s", target.App)
				}
				libIndex := FindAppLibraryIndex(config.Apps[appIndex], name)
				if libIndex == -1 {
					return installDependency{}, fmt.Errorf("library not registered in workspace: %s", name)
				}
				matches = append(matches, installDependency{
					kind: dependencyAppLib,
					app:  target.App,
					name: name,
					path: appLibPath,
				})
			}
		}
	}

	if lib, ok, err := FindGlobalLibraryByName(config, name); err != nil {
		return installDependency{}, err
	} else if ok {
		depPath := filepath.Join(root, "repos", "libs", lib.Name)
		matches = append(matches, installDependency{
			kind: dependencyGlobalLib,
			name: name,
			path: depPath,
		})
	}

	if project, ok, err := FindProjectByName(config, name); err != nil {
		return installDependency{}, err
	} else if ok {
		depPath := filepath.Join(root, "repos", "projects", project.Name)
		matches = append(matches, installDependency{
			kind: dependencyProject,
			name: name,
			path: depPath,
		})
	}

	if len(matches) == 0 {
		if target.Kind == targetService || target.Kind == targetAppLib || target.Kind == targetTest {
			servicePath := filepath.Join(root, "repos", "apps", target.App, "services", name)
			if DirExists(afero.NewOsFs(), servicePath) {
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
		case targetTest:
			return fmt.Errorf("invalid dependency for test")
		default:
			return fmt.Errorf("unsupported install target")
		}
	}
	if dependency.kind == dependencyAppLib && target.App != dependency.app {
		return fmt.Errorf("app-scoped libraries must stay within the same app")
	}
	return nil
}

func promptForDependency(config WorkspaceConfig, root string) (installDependency, error) {
	options := []string{}
	if len(config.Libraries) > 0 {
		options = append(options, "global library")
	}
	appNames := AppNamesWithLibraries(config)
	if len(appNames) > 0 {
		options = append(options, "app library")
	}
	if len(options) == 0 {
		return installDependency{}, fmt.Errorf("no libraries available to install")
	}
	scope := options[0]
	if len(options) > 1 {
		selected, err := SelectFromList("Select payload source", options)
		if err != nil {
			return installDependency{}, err
		}
		scope = selected
	}
	if scope == "app library" {
		appName, err := SelectFromList("Select app", appNames)
		if err != nil {
			return installDependency{}, err
		}
		appLibs := AppLibraryNames(config, appName)
		if len(appLibs) == 0 {
			return installDependency{}, fmt.Errorf("no libraries found for app: %s", appName)
		}
		libName, err := SelectFromList("Select library", appLibs)
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
	globalLibs := GlobalLibraryNames(config)
	libName, err := SelectFromList("Select library", globalLibs)
	if err != nil {
		return installDependency{}, err
	}
	return installDependency{
		kind: dependencyGlobalLib,
		name: libName,
		path: filepath.Join(root, "repos", "libs", libName),
	}, nil
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

func registerDependency(config WorkspaceConfig, target installTarget, dependency installDependency) (WorkspaceConfig, error) {
	targetKey := installTargetKey(target)
	depKey := installDependencyKey(dependency)
	if targetKey == depKey {
		return WorkspaceConfig{}, fmt.Errorf("dependency cannot reference itself")
	}

	switch target.Kind {
	case targetService:
		appIndex := FindAppIndex(config, target.App)
		if appIndex == -1 {
			return WorkspaceConfig{}, fmt.Errorf("app not registered in workspace: %s", target.App)
		}
		serviceIndex := FindServiceIndex(config.Apps[appIndex], target.Name)
		if serviceIndex == -1 {
			return WorkspaceConfig{}, fmt.Errorf("service not registered in workspace: %s", target.Name)
		}
		depsList := config.Apps[appIndex].Services[serviceIndex].Deps
		if containsDep(depsList, dependency.name, depSourceForInstall(dependency)) {
			return WorkspaceConfig{}, fmt.Errorf("dependency already registered: %s", dependency.name)
		}
		config.Apps[appIndex].Services[serviceIndex].Deps = append(depsList, makeInstallDep(dependency))
	case targetAppLib:
		appIndex := FindAppIndex(config, target.App)
		if appIndex == -1 {
			return WorkspaceConfig{}, fmt.Errorf("app not registered in workspace: %s", target.App)
		}
		libIndex := FindAppLibraryIndex(config.Apps[appIndex], target.Name)
		if libIndex == -1 {
			return WorkspaceConfig{}, fmt.Errorf("library not registered in workspace: %s", target.Name)
		}
		depsList := config.Apps[appIndex].Libraries[libIndex].Deps
		if containsDep(depsList, dependency.name, depSourceForInstall(dependency)) {
			return WorkspaceConfig{}, fmt.Errorf("dependency already registered: %s", dependency.name)
		}
		if err := ensureNoCycles(config, target, dependency); err != nil {
			return WorkspaceConfig{}, err
		}
		config.Apps[appIndex].Libraries[libIndex].Deps = append(depsList, makeInstallDep(dependency))
	case targetTest:
		appIndex := FindAppIndex(config, target.App)
		if appIndex == -1 {
			return WorkspaceConfig{}, fmt.Errorf("app not registered in workspace: %s", target.App)
		}
		testIndex := FindAppTestIndex(config.Apps[appIndex], target.Name)
		if testIndex == -1 {
			return WorkspaceConfig{}, fmt.Errorf("test not registered in workspace: %s", target.Name)
		}
		depsList := config.Apps[appIndex].Testing[testIndex].Deps
		if containsDep(depsList, dependency.name, depSourceForInstall(dependency)) {
			return WorkspaceConfig{}, fmt.Errorf("dependency already registered: %s", dependency.name)
		}
		config.Apps[appIndex].Testing[testIndex].Deps = append(depsList, makeInstallDep(dependency))
	case targetGlobalLib:
		libIndex := FindGlobalLibraryIndex(config, target.Name)
		if libIndex == -1 {
			return WorkspaceConfig{}, fmt.Errorf("library not registered in workspace: %s", target.Name)
		}
		depsList := config.Libraries[libIndex].Deps
		if containsDep(depsList, dependency.name, depSourceForInstall(dependency)) {
			return WorkspaceConfig{}, fmt.Errorf("dependency already registered: %s", dependency.name)
		}
		if err := ensureNoCycles(config, target, dependency); err != nil {
			return WorkspaceConfig{}, err
		}
		config.Libraries[libIndex].Deps = append(depsList, makeInstallDep(dependency))
	case targetProject:
		projectIndex := FindProjectIndex(config, target.Name)
		if projectIndex == -1 {
			return WorkspaceConfig{}, fmt.Errorf("project not registered in workspace: %s", target.Name)
		}
		depsList := config.Projects[projectIndex].Deps
		if containsDep(depsList, dependency.name, depSourceForInstall(dependency)) {
			return WorkspaceConfig{}, fmt.Errorf("dependency already registered: %s", dependency.name)
		}
		if err := ensureNoCycles(config, target, dependency); err != nil {
			return WorkspaceConfig{}, err
		}
		config.Projects[projectIndex].Deps = append(depsList, makeInstallDep(dependency))
	default:
		return WorkspaceConfig{}, fmt.Errorf("unsupported install target")
	}

	return config, nil
}

func ensureNoCycles(config WorkspaceConfig, target installTarget, dependency installDependency) error {
	if target.Kind == targetService || target.Kind == targetTest {
		return nil
	}
	graph, err := BuildDependencyGraph(config)
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
	if HasPath(graph, depKey, targetKey) {
		return fmt.Errorf("dependency would create a cycle")
	}
	return nil
}

func installTargetKey(target installTarget) string {
	switch target.Kind {
	case targetService:
		return ServiceKey(target.App, target.Name)
	case targetAppLib:
		return AppLibKey(target.App, target.Name)
	case targetTest:
		return fmt.Sprintf("test:%s:%s", target.App, target.Name)
	case targetGlobalLib:
		return GlobalLibKey(target.Name)
	case targetProject:
		return ProjectKey(target.Name)
	default:
		return ""
	}
}

func installDependencyKey(dep installDependency) string {
	switch dep.kind {
	case dependencyAppLib:
		return AppLibKey(dep.app, dep.name)
	case dependencyGlobalLib:
		return GlobalLibKey(dep.name)
	case dependencyProject:
		return ProjectKey(dep.name)
	default:
		return ""
	}
}

func containsDep(values []WorkspaceDep, lib string, from string) bool {
	for _, value := range values {
		if value.Lib == lib && value.From == from {
			return true
		}
	}
	return false
}

func makeInstallDep(dep installDependency) WorkspaceDep {
	return WorkspaceDep{
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
