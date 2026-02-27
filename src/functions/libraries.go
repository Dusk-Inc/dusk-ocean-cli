package functions

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/afero"
)

func FindGlobalLib(fs afero.Fs, root string, name string) (bool, string, error) {
	libPath := filepath.Join(root, "repos", "libs", name)
	return FindRepoLanguage(fs, libPath)
}

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

func hasLibraryDep(depsList []WorkspaceDep, name string, source string) bool {
	for _, dep := range depsList {
		if dep.Lib == name && dep.From == source {
			return true
		}
	}
	return false
}

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
