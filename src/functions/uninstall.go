package functions

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

type uninstallDependencyKind string

const (
	uninstallDependencyGlobalLib uninstallDependencyKind = "global-lib"
	uninstallDependencyAppLib    uninstallDependencyKind = "app-lib"
	uninstallDependencyProject   uninstallDependencyKind = "project"
)

type uninstallDependency struct {
	kind uninstallDependencyKind
	app  string
	name string
	from string
	path string
}

func RunUninstallPrompt(cmd *cobra.Command, fs afero.Fs) error {
	root, err := EnsureWorkspaceRoot(fs)
	if err != nil {
		return err
	}
	config, err := ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	target, err := PromptForTarget(config, root)
	if err != nil {
		return err
	}
	depsList, err := collectTargetDeps(config, target)
	if err != nil {
		return err
	}
	if len(depsList) == 0 {
		return fmt.Errorf("no dependencies available for uninstall")
	}
	dependency, err := promptForUninstallDependency(config, root, depsList)
	if err != nil {
		return err
	}
	confirmPrompt := promptui.Prompt{
		Label:     fmt.Sprintf("Uninstall %s from %s", formatUninstallDependencyLabel(dependency), FormatTargetLabel(target)),
		IsConfirm: true,
	}
	confirm, err := confirmPrompt.Run()
	if err != nil {
		return err
	}
	if strings.ToLower(confirm) != "y" {
		return fmt.Errorf("aborted")
	}
	return runUninstall(cmd, fs, target, dependency, UninstallOptions{})
}

func runUninstall(cmd *cobra.Command, fs afero.Fs, target Target, dependency uninstallDependency, options UninstallOptions) error {
	if err := RunUninstallForTargets(cmd, fs, dependency.path, dependency.name, []Target{target}, options); err != nil {
		return err
	}
	return UpdateConfig(fs, func(config WorkspaceConfig) (WorkspaceConfig, error) {
		return removeTargetDependency(config, target, dependency), nil
	})
}

func collectTargetDeps(config WorkspaceConfig, target Target) ([]WorkspaceDep, error) {
	switch target.Kind {
	case TargetService:
		appIndex := FindAppIndex(config, target.App)
		if appIndex == -1 {
			return nil, fmt.Errorf("app not registered in workspace: %s", target.App)
		}
		serviceIndex := FindServiceIndex(config.Apps[appIndex], target.Name)
		if serviceIndex == -1 {
			return nil, fmt.Errorf("service not registered in workspace: %s", target.Name)
		}
		return config.Apps[appIndex].Services[serviceIndex].Deps, nil
	case TargetAppLib:
		appIndex := FindAppIndex(config, target.App)
		if appIndex == -1 {
			return nil, fmt.Errorf("app not registered in workspace: %s", target.App)
		}
		libIndex := FindAppLibraryIndex(config.Apps[appIndex], target.Name)
		if libIndex == -1 {
			return nil, fmt.Errorf("library not registered in workspace: %s", target.Name)
		}
		return config.Apps[appIndex].Libraries[libIndex].Deps, nil
	case TargetGlobalLib:
		libIndex := FindGlobalLibraryIndex(config, target.Name)
		if libIndex == -1 {
			return nil, fmt.Errorf("library not registered in workspace: %s", target.Name)
		}
		return config.Libraries[libIndex].Deps, nil
	case TargetProject:
		projectIndex := FindProjectIndex(config, target.Name)
		if projectIndex == -1 {
			return nil, fmt.Errorf("project not registered in workspace: %s", target.Name)
		}
		return config.Projects[projectIndex].Deps, nil
	case TargetTest:
		appIndex := FindAppIndex(config, target.App)
		if appIndex == -1 {
			return nil, fmt.Errorf("app not registered in workspace: %s", target.App)
		}
		testIndex := FindAppTestIndex(config.Apps[appIndex], target.Name)
		if testIndex == -1 {
			return nil, fmt.Errorf("test not registered in workspace: %s", target.Name)
		}
		return config.Apps[appIndex].Testing[testIndex].Deps, nil
	default:
		return nil, fmt.Errorf("unsupported uninstall target")
	}
}

func promptForUninstallDependency(config WorkspaceConfig, root string, depsList []WorkspaceDep) (uninstallDependency, error) {
	options := make([]string, 0, len(depsList))
	entries := make([]uninstallDependency, 0, len(depsList))
	for _, dep := range depsList {
		entry, err := resolveUninstallDependency(config, root, dep)
		if err != nil {
			return uninstallDependency{}, err
		}
		options = append(options, formatUninstallDependencyLabel(entry))
		entries = append(entries, entry)
	}
	selected, err := SelectFromList("Select dependency", options)
	if err != nil {
		return uninstallDependency{}, err
	}
	for i, option := range options {
		if option == selected {
			return entries[i], nil
		}
	}
	return uninstallDependency{}, fmt.Errorf("selected dependency not found")
}

func resolveUninstallDependency(config WorkspaceConfig, root string, dep WorkspaceDep) (uninstallDependency, error) {
	switch dep.From {
	case "global":
		if _, ok, err := FindGlobalLibraryByName(config, dep.Lib); err != nil {
			return uninstallDependency{}, err
		} else if !ok {
			return uninstallDependency{}, fmt.Errorf("library not registered in workspace: %s", dep.Lib)
		}
		return uninstallDependency{
			kind: uninstallDependencyGlobalLib,
			name: dep.Lib,
			from: dep.From,
			path: filepath.Join(root, "repos", "libs", dep.Lib),
		}, nil
	case "project":
		if _, ok, err := FindProjectByName(config, dep.Lib); err != nil {
			return uninstallDependency{}, err
		} else if !ok {
			return uninstallDependency{}, fmt.Errorf("project not registered in workspace: %s", dep.Lib)
		}
		return uninstallDependency{
			kind: uninstallDependencyProject,
			name: dep.Lib,
			from: dep.From,
			path: filepath.Join(root, "repos", "projects", dep.Lib),
		}, nil
	default:
		appIndex := FindAppIndex(config, dep.From)
		if appIndex == -1 {
			return uninstallDependency{}, fmt.Errorf("app not registered in workspace: %s", dep.From)
		}
		if FindAppLibraryIndex(config.Apps[appIndex], dep.Lib) == -1 {
			return uninstallDependency{}, fmt.Errorf("library not registered in workspace: %s", dep.Lib)
		}
		return uninstallDependency{
			kind: uninstallDependencyAppLib,
			app:  dep.From,
			name: dep.Lib,
			from: dep.From,
			path: filepath.Join(root, "repos", "apps", dep.From, "libs", dep.Lib),
		}, nil
	}
}

func formatUninstallDependencyLabel(dep uninstallDependency) string {
	switch dep.kind {
	case uninstallDependencyAppLib:
		return fmt.Sprintf("app library %s/%s", dep.app, dep.name)
	case uninstallDependencyGlobalLib:
		return fmt.Sprintf("global library %s", dep.name)
	case uninstallDependencyProject:
		return fmt.Sprintf("project %s", dep.name)
	default:
		return dep.name
	}
}

func removeTargetDependency(config WorkspaceConfig, target Target, dependency uninstallDependency) WorkspaceConfig {
	switch target.Kind {
	case TargetService:
		appIndex := FindAppIndex(config, target.App)
		if appIndex == -1 {
			return config
		}
		serviceIndex := FindServiceIndex(config.Apps[appIndex], target.Name)
		if serviceIndex == -1 {
			return config
		}
		depsList := config.Apps[appIndex].Services[serviceIndex].Deps
		config.Apps[appIndex].Services[serviceIndex].Deps = filterTargetDeps(depsList, dependency.name, dependency.from)
	case TargetAppLib:
		appIndex := FindAppIndex(config, target.App)
		if appIndex == -1 {
			return config
		}
		libIndex := FindAppLibraryIndex(config.Apps[appIndex], target.Name)
		if libIndex == -1 {
			return config
		}
		depsList := config.Apps[appIndex].Libraries[libIndex].Deps
		config.Apps[appIndex].Libraries[libIndex].Deps = filterTargetDeps(depsList, dependency.name, dependency.from)
	case TargetGlobalLib:
		libIndex := FindGlobalLibraryIndex(config, target.Name)
		if libIndex == -1 {
			return config
		}
		depsList := config.Libraries[libIndex].Deps
		config.Libraries[libIndex].Deps = filterTargetDeps(depsList, dependency.name, dependency.from)
	case TargetProject:
		projectIndex := FindProjectIndex(config, target.Name)
		if projectIndex == -1 {
			return config
		}
		depsList := config.Projects[projectIndex].Deps
		config.Projects[projectIndex].Deps = filterTargetDeps(depsList, dependency.name, dependency.from)
	case TargetTest:
		appIndex := FindAppIndex(config, target.App)
		if appIndex == -1 {
			return config
		}
		testIndex := FindAppTestIndex(config.Apps[appIndex], target.Name)
		if testIndex == -1 {
			return config
		}
		depsList := config.Apps[appIndex].Testing[testIndex].Deps
		config.Apps[appIndex].Testing[testIndex].Deps = filterTargetDeps(depsList, dependency.name, dependency.from)
	}
	return config
}

func filterTargetDeps(depsList []WorkspaceDep, name string, source string) []WorkspaceDep {
	if len(depsList) == 0 {
		return depsList
	}
	updated := make([]WorkspaceDep, 0, len(depsList))
	for _, dep := range depsList {
		if dep.Lib == name && dep.From == source {
			continue
		}
		updated = append(updated, dep)
	}
	return updated
}
