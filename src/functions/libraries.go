package functions

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/afero"
)

// FindGlobalLib reports the detected language of a global library repo.
func FindGlobalLib(fs afero.Fs, root string, name string) (bool, string, error) {
	libPath := filepath.Join(root, "repos", "libs", name)
	return FindRepoLanguage(fs, libPath)
}

// MakeAppLibNode builds the dependency-graph node for a registered app-scoped library.
func MakeAppLibNode(config WorkspaceConfig, appName string, name string) (Node, error) {
	lib, ok := FindAppLibraryByName(config, appName, name)
	if !ok {
		return Node{}, fmt.Errorf("library not registered in workspace: %s", name)
	}
	return Node{
		Kind: NodeAppLib,
		App:  appName,
		Name: name,
		Deps: lib.Deps,
	}, nil
}

// MakeGlobalLibNode builds the dependency-graph node for a registered global library.
func MakeGlobalLibNode(config WorkspaceConfig, name string) (Node, error) {
	lib, ok, err := FindGlobalLibraryByName(config, name)
	if err != nil {
		return Node{}, err
	}
	if !ok {
		return Node{}, fmt.Errorf("library not registered in workspace: %s", name)
	}
	return Node{
		Kind: NodeGlobalLib,
		Name: lib.Name,
		Deps: lib.Deps,
	}, nil
}

// AddAppLibraryToWorkspace registers a library under an app, creating the app entry when absent.
func AddAppLibraryToWorkspace(fs afero.Fs, appName string, name string) error {
	config, err := ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	appIndex := FindAppIndex(config, appName)
	if appIndex == -1 {
		config.Apps = append(config.Apps, WorkspaceApp{
			Name: appName,
			Libraries: []WorkspaceLibrary{
				{
					Name: name,
					Deps: []WorkspaceDep{},
				},
			},
			Services: []WorkspaceService{},
			Testing:  []WorkspaceTest{},
		})
		return WriteWorkspaceConfig(fs, config)
	}
	if FindAppLibraryIndex(config.Apps[appIndex], name) != -1 {
		return nil
	}
	config.Apps[appIndex].Libraries = append(config.Apps[appIndex].Libraries, WorkspaceLibrary{
		Name: name,
		Deps: []WorkspaceDep{},
	})
	return WriteWorkspaceConfig(fs, config)
}

// AddGlobalLibraryToWorkspace registers a global library, no-op when it is already present.
func AddGlobalLibraryToWorkspace(fs afero.Fs, name string) error {
	config, err := ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	if FindGlobalLibraryIndex(config, name) != -1 {
		return nil
	}
	config.Libraries = append(config.Libraries, WorkspaceLibrary{
		Name: name,
		Deps: []WorkspaceDep{},
	})
	return WriteWorkspaceConfig(fs, config)
}

// RemoveAppLibraryFromWorkspace unregisters a library from its app, no-op when absent.
func RemoveAppLibraryFromWorkspace(fs afero.Fs, appName string, name string) error {
	return UpdateConfig(fs, func(config WorkspaceConfig) (WorkspaceConfig, error) {
		appIndex := FindAppIndex(config, appName)
		if appIndex == -1 {
			return config, nil
		}
		libIndex := FindAppLibraryIndex(config.Apps[appIndex], name)
		if libIndex == -1 {
			return config, nil
		}
		libs := config.Apps[appIndex].Libraries
		config.Apps[appIndex].Libraries = append(libs[:libIndex], libs[libIndex+1:]...)
		return config, nil
	})
}

// RemoveGlobalLibraryFromWorkspace unregisters a global library, no-op when absent.
func RemoveGlobalLibraryFromWorkspace(fs afero.Fs, name string) error {
	return UpdateConfig(fs, func(config WorkspaceConfig) (WorkspaceConfig, error) {
		removed := false
		updated := make([]WorkspaceLibrary, 0, len(config.Libraries))
		for _, lib := range config.Libraries {
			if lib.Name == name {
				removed = true
				continue
			}
			updated = append(updated, lib)
		}
		if !removed {
			return config, nil
		}
		config.Libraries = updated
		return config, nil
	})
}

// CollectLibraryDependents returns every registered target that declares a dependency on the named library from the given source.
func CollectLibraryDependents(config WorkspaceConfig, root string, name string, source string) []Target {
	var targets []Target
	for _, app := range config.Apps {
		for _, service := range app.Services {
			if hasLibraryDep(service.Deps, name, source) {
				targets = append(targets, Target{
					Kind: TargetService,
					App:  app.Name,
					Name: service.Name,
					Path: filepath.Join(root, "repos", "apps", app.Name, "services", service.Name),
				})
			}
		}
		for _, lib := range app.Libraries {
			if hasLibraryDep(lib.Deps, name, source) {
				targets = append(targets, Target{
					Kind: TargetAppLib,
					App:  app.Name,
					Name: lib.Name,
					Path: filepath.Join(root, "repos", "apps", app.Name, "libs", lib.Name),
				})
			}
		}
		for _, project := range app.Projects {
			if hasLibraryDep(project.Deps, name, source) {
				targets = append(targets, Target{
					Kind: TargetAppProject,
					App:  app.Name,
					Name: project.Name,
					Path: filepath.Join(root, "repos", "apps", app.Name, "projects", project.Name),
				})
			}
		}
		for _, test := range app.Testing {
			if hasLibraryDep(test.Deps, name, source) {
				targets = append(targets, Target{
					Kind: TargetTest,
					App:  app.Name,
					Name: test.Name,
					Path: filepath.Join(root, "repos", "apps", app.Name, "testing", test.Name),
				})
			}
		}
	}
	for _, lib := range config.Libraries {
		if hasLibraryDep(lib.Deps, name, source) {
			targets = append(targets, Target{
				Kind: TargetGlobalLib,
				Name: lib.Name,
				Path: filepath.Join(root, "repos", "libs", lib.Name),
			})
		}
	}
	for _, project := range config.Projects {
		if hasLibraryDep(project.Deps, name, source) {
			targets = append(targets, Target{
				Kind: TargetProject,
				Name: project.Name,
				Path: filepath.Join(root, "repos", "projects", project.Name),
			})
		}
	}
	return targets
}

// RemoveLibraryDeps strips the named library from every target's dependency list.
func RemoveLibraryDeps(config WorkspaceConfig, name string, source string) WorkspaceConfig {
	for appIndex := range config.Apps {
		for serviceIndex := range config.Apps[appIndex].Services {
			depsList := config.Apps[appIndex].Services[serviceIndex].Deps
			config.Apps[appIndex].Services[serviceIndex].Deps = filterLibraryDeps(depsList, name, source)
		}
		for libIndex := range config.Apps[appIndex].Libraries {
			depsList := config.Apps[appIndex].Libraries[libIndex].Deps
			config.Apps[appIndex].Libraries[libIndex].Deps = filterLibraryDeps(depsList, name, source)
		}
		for projectIndex := range config.Apps[appIndex].Projects {
			depsList := config.Apps[appIndex].Projects[projectIndex].Deps
			config.Apps[appIndex].Projects[projectIndex].Deps = filterLibraryDeps(depsList, name, source)
		}
		for testIndex := range config.Apps[appIndex].Testing {
			depsList := config.Apps[appIndex].Testing[testIndex].Deps
			config.Apps[appIndex].Testing[testIndex].Deps = filterLibraryDeps(depsList, name, source)
		}
	}
	for libIndex := range config.Libraries {
		depsList := config.Libraries[libIndex].Deps
		config.Libraries[libIndex].Deps = filterLibraryDeps(depsList, name, source)
	}
	for projectIndex := range config.Projects {
		depsList := config.Projects[projectIndex].Deps
		config.Projects[projectIndex].Deps = filterLibraryDeps(depsList, name, source)
	}
	return config
}

// RemoveAppLibraryFromConfig drops an app-scoped library's entry from a config value, returning the result.
func RemoveAppLibraryFromConfig(config WorkspaceConfig, appName string, name string) WorkspaceConfig {
	appIndex := FindAppIndex(config, appName)
	if appIndex == -1 {
		return config
	}
	libIndex := FindAppLibraryIndex(config.Apps[appIndex], name)
	if libIndex == -1 {
		return config
	}
	libs := config.Apps[appIndex].Libraries
	config.Apps[appIndex].Libraries = append(libs[:libIndex], libs[libIndex+1:]...)
	return config
}

// RemoveGlobalLibraryFromConfig drops a global library's entry from a config value, returning the result.
func RemoveGlobalLibraryFromConfig(config WorkspaceConfig, name string) WorkspaceConfig {
	updated := make([]WorkspaceLibrary, 0, len(config.Libraries))
	for _, lib := range config.Libraries {
		if lib.Name == name {
			continue
		}
		updated = append(updated, lib)
	}
	config.Libraries = updated
	return config
}

// hasLibraryDep reports whether a dependency list carries the named library from the given source.
func hasLibraryDep(depsList []WorkspaceDep, name string, source string) bool {
	for _, dep := range depsList {
		if dep.Lib == name && dep.From == source {
			return true
		}
	}
	return false
}

// filterLibraryDeps returns the dependency list with the named library from the given source removed.
func filterLibraryDeps(depsList []WorkspaceDep, name string, source string) []WorkspaceDep {
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
