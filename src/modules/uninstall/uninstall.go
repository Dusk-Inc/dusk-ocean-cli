package uninstall

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/deps"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/prompts"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/targets"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/workspace"
	"github.com/manifoldco/promptui"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

type dependencyKind string

const (
	dependencyGlobalLib dependencyKind = "global-lib"
	dependencyAppLib    dependencyKind = "app-lib"
	dependencyProject   dependencyKind = "project"
)

type uninstallDependency struct {
	kind dependencyKind
	app  string
	name string
	from string
	path string
}

func RunUninstallPrompt(cmd *cobra.Command, fs afero.Fs) error {
	root, err := workspace.EnsureWorkspaceRoot(fs)
	if err != nil {
		return err
	}
	config, err := workspace.ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	target, err := prompts.PromptForTarget(config, root)
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
	dependency, err := promptForDependency(config, root, depsList)
	if err != nil {
		return err
	}
	confirmPrompt := promptui.Prompt{
		Label:     fmt.Sprintf("Uninstall %s from %s", formatDependencyLabel(dependency), targets.FormatTargetLabel(target)),
		IsConfirm: true,
	}
	confirm, err := confirmPrompt.Run()
	if err != nil {
		return err
	}
	if strings.ToLower(confirm) != "y" {
		return fmt.Errorf("aborted")
	}
	return runUninstall(cmd, fs, target, dependency, deps.UninstallOptions{})
}

func runUninstall(cmd *cobra.Command, fs afero.Fs, target workspace.Target, dependency uninstallDependency, options deps.UninstallOptions) error {
	if err := deps.RunUninstallForTargets(cmd, fs, dependency.path, dependency.name, []workspace.Target{target}, options); err != nil {
		return err
	}
	return workspace.UpdateConfig(fs, func(config workspace.WorkspaceConfig) (workspace.WorkspaceConfig, error) {
		return removeTargetDependency(config, target, dependency), nil
	})
}

func collectTargetDeps(config workspace.WorkspaceConfig, target workspace.Target) ([]workspace.WorkspaceDep, error) {
	switch target.Kind {
	case workspace.TargetService:
		appIndex := workspace.FindAppIndex(config, target.App)
		if appIndex == -1 {
			return nil, fmt.Errorf("app not registered in workspace: %s", target.App)
		}
		serviceIndex := workspace.FindServiceIndex(config.Apps[appIndex], target.Name)
		if serviceIndex == -1 {
			return nil, fmt.Errorf("service not registered in workspace: %s", target.Name)
		}
		return config.Apps[appIndex].Services[serviceIndex].Deps, nil
	case workspace.TargetAppLib:
		appIndex := workspace.FindAppIndex(config, target.App)
		if appIndex == -1 {
			return nil, fmt.Errorf("app not registered in workspace: %s", target.App)
		}
		libIndex := workspace.FindAppLibraryIndex(config.Apps[appIndex], target.Name)
		if libIndex == -1 {
			return nil, fmt.Errorf("library not registered in workspace: %s", target.Name)
		}
		return config.Apps[appIndex].Libraries[libIndex].Deps, nil
	case workspace.TargetGlobalLib:
		libIndex := workspace.FindGlobalLibraryIndex(config, target.Name)
		if libIndex == -1 {
			return nil, fmt.Errorf("library not registered in workspace: %s", target.Name)
		}
		return config.Libraries[libIndex].Deps, nil
	case workspace.TargetProject:
		projectIndex := workspace.FindProjectIndex(config, target.Name)
		if projectIndex == -1 {
			return nil, fmt.Errorf("project not registered in workspace: %s", target.Name)
		}
		return config.Projects[projectIndex].Deps, nil
	case workspace.TargetTest:
		appIndex := workspace.FindAppIndex(config, target.App)
		if appIndex == -1 {
			return nil, fmt.Errorf("app not registered in workspace: %s", target.App)
		}
		testIndex := workspace.FindAppTestIndex(config.Apps[appIndex], target.Name)
		if testIndex == -1 {
			return nil, fmt.Errorf("test not registered in workspace: %s", target.Name)
		}
		return config.Apps[appIndex].Testing[testIndex].Deps, nil
	default:
		return nil, fmt.Errorf("unsupported uninstall target")
	}
}

func promptForDependency(config workspace.WorkspaceConfig, root string, depsList []workspace.WorkspaceDep) (uninstallDependency, error) {
	options := make([]string, 0, len(depsList))
	entries := make([]uninstallDependency, 0, len(depsList))
	for _, dep := range depsList {
		entry, err := resolveDependency(config, root, dep)
		if err != nil {
			return uninstallDependency{}, err
		}
		options = append(options, formatDependencyLabel(entry))
		entries = append(entries, entry)
	}
	selected, err := prompts.SelectFromList("Select dependency", options)
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

func resolveDependency(config workspace.WorkspaceConfig, root string, dep workspace.WorkspaceDep) (uninstallDependency, error) {
	switch dep.From {
	case "global":
		if _, ok, err := workspace.FindGlobalLibraryByName(config, dep.Lib); err != nil {
			return uninstallDependency{}, err
		} else if !ok {
			return uninstallDependency{}, fmt.Errorf("library not registered in workspace: %s", dep.Lib)
		}
		return uninstallDependency{
			kind: dependencyGlobalLib,
			name: dep.Lib,
			from: dep.From,
			path: filepath.Join(root, "repos", "libs", dep.Lib),
		}, nil
	case "project":
		if _, ok, err := workspace.FindProjectByName(config, dep.Lib); err != nil {
			return uninstallDependency{}, err
		} else if !ok {
			return uninstallDependency{}, fmt.Errorf("project not registered in workspace: %s", dep.Lib)
		}
		return uninstallDependency{
			kind: dependencyProject,
			name: dep.Lib,
			from: dep.From,
			path: filepath.Join(root, "repos", "projects", dep.Lib),
		}, nil
	default:
		appIndex := workspace.FindAppIndex(config, dep.From)
		if appIndex == -1 {
			return uninstallDependency{}, fmt.Errorf("app not registered in workspace: %s", dep.From)
		}
		if workspace.FindAppLibraryIndex(config.Apps[appIndex], dep.Lib) == -1 {
			return uninstallDependency{}, fmt.Errorf("library not registered in workspace: %s", dep.Lib)
		}
		return uninstallDependency{
			kind: dependencyAppLib,
			app:  dep.From,
			name: dep.Lib,
			from: dep.From,
			path: filepath.Join(root, "repos", "apps", dep.From, "libs", dep.Lib),
		}, nil
	}
}

func formatDependencyLabel(dep uninstallDependency) string {
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

func removeTargetDependency(config workspace.WorkspaceConfig, target workspace.Target, dependency uninstallDependency) workspace.WorkspaceConfig {
	switch target.Kind {
	case workspace.TargetService:
		appIndex := workspace.FindAppIndex(config, target.App)
		if appIndex == -1 {
			return config
		}
		serviceIndex := workspace.FindServiceIndex(config.Apps[appIndex], target.Name)
		if serviceIndex == -1 {
			return config
		}
		depsList := config.Apps[appIndex].Services[serviceIndex].Deps
		config.Apps[appIndex].Services[serviceIndex].Deps = filterTargetDeps(depsList, dependency.name, dependency.from)
	case workspace.TargetAppLib:
		appIndex := workspace.FindAppIndex(config, target.App)
		if appIndex == -1 {
			return config
		}
		libIndex := workspace.FindAppLibraryIndex(config.Apps[appIndex], target.Name)
		if libIndex == -1 {
			return config
		}
		depsList := config.Apps[appIndex].Libraries[libIndex].Deps
		config.Apps[appIndex].Libraries[libIndex].Deps = filterTargetDeps(depsList, dependency.name, dependency.from)
	case workspace.TargetGlobalLib:
		libIndex := workspace.FindGlobalLibraryIndex(config, target.Name)
		if libIndex == -1 {
			return config
		}
		depsList := config.Libraries[libIndex].Deps
		config.Libraries[libIndex].Deps = filterTargetDeps(depsList, dependency.name, dependency.from)
	case workspace.TargetProject:
		projectIndex := workspace.FindProjectIndex(config, target.Name)
		if projectIndex == -1 {
			return config
		}
		depsList := config.Projects[projectIndex].Deps
		config.Projects[projectIndex].Deps = filterTargetDeps(depsList, dependency.name, dependency.from)
	case workspace.TargetTest:
		appIndex := workspace.FindAppIndex(config, target.App)
		if appIndex == -1 {
			return config
		}
		testIndex := workspace.FindAppTestIndex(config.Apps[appIndex], target.Name)
		if testIndex == -1 {
			return config
		}
		depsList := config.Apps[appIndex].Testing[testIndex].Deps
		config.Apps[appIndex].Testing[testIndex].Deps = filterTargetDeps(depsList, dependency.name, dependency.from)
	}
	return config
}

func filterTargetDeps(depsList []workspace.WorkspaceDep, name string, source string) []workspace.WorkspaceDep {
	if len(depsList) == 0 {
		return depsList
	}
	updated := make([]workspace.WorkspaceDep, 0, len(depsList))
	for _, dep := range depsList {
		if dep.Lib == name && dep.From == source {
			continue
		}
		updated = append(updated, dep)
	}
	return updated
}
