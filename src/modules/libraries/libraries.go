package libraries

import (
	"fmt"
	"path/filepath"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/deps"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/workspace"
	"github.com/spf13/afero"
)

func FindGlobalLib(fs afero.Fs, root string, name string) (bool, string, error) {
	libPath := filepath.Join(root, "repos", "libs", name)
	return workspace.FindRepoLanguage(fs, libPath)
}

func MakeAppLibNode(config workspace.WorkspaceConfig, appName string, name string) (deps.Node, error) {
	lib, ok := workspace.FindAppLibraryByName(config, appName, name)
	if !ok {
		return deps.Node{}, fmt.Errorf("library not registered in workspace: %s", name)
	}
	return deps.Node{
		Kind: deps.NodeAppLib,
		App:  appName,
		Name: name,
		Deps: lib.Deps,
	}, nil
}

func MakeGlobalLibNode(config workspace.WorkspaceConfig, name string) (deps.Node, error) {
	lib, ok, err := workspace.FindGlobalLibraryByName(config, name)
	if err != nil {
		return deps.Node{}, err
	}
	if !ok {
		return deps.Node{}, fmt.Errorf("library not registered in workspace: %s", name)
	}
	return deps.Node{
		Kind: deps.NodeGlobalLib,
		Name: lib.Name,
		Deps: lib.Deps,
	}, nil
}

func AddAppLibraryToWorkspace(fs afero.Fs, appName string, name string) error {
	config, err := workspace.ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	appIndex := workspace.FindAppIndex(config, appName)
	if appIndex == -1 {
		config.Apps = append(config.Apps, workspace.WorkspaceApp{
			Name: appName,
			Libraries: []workspace.WorkspaceLibrary{
				{
					Name: name,
					Deps: []workspace.WorkspaceDep{},
				},
			},
			Services: []workspace.WorkspaceService{},
			Testing:  []workspace.WorkspaceTest{},
		})
		return workspace.WriteWorkspaceConfig(fs, config)
	}
	if workspace.FindAppLibraryIndex(config.Apps[appIndex], name) != -1 {
		return nil
	}
	config.Apps[appIndex].Libraries = append(config.Apps[appIndex].Libraries, workspace.WorkspaceLibrary{
		Name: name,
		Deps: []workspace.WorkspaceDep{},
	})
	return workspace.WriteWorkspaceConfig(fs, config)
}

func AddGlobalLibraryToWorkspace(fs afero.Fs, name string) error {
	config, err := workspace.ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	if workspace.FindGlobalLibraryIndex(config, name) != -1 {
		return nil
	}
	config.Libraries = append(config.Libraries, workspace.WorkspaceLibrary{
		Name: name,
		Deps: []workspace.WorkspaceDep{},
	})
	return workspace.WriteWorkspaceConfig(fs, config)
}

func RemoveAppLibraryFromWorkspace(fs afero.Fs, appName string, name string) error {
	return workspace.UpdateConfig(fs, func(config workspace.WorkspaceConfig) (workspace.WorkspaceConfig, error) {
		appIndex := workspace.FindAppIndex(config, appName)
		if appIndex == -1 {
			return config, nil
		}
		libIndex := workspace.FindAppLibraryIndex(config.Apps[appIndex], name)
		if libIndex == -1 {
			return config, nil
		}
		libs := config.Apps[appIndex].Libraries
		config.Apps[appIndex].Libraries = append(libs[:libIndex], libs[libIndex+1:]...)
		return config, nil
	})
}

func RemoveGlobalLibraryFromWorkspace(fs afero.Fs, name string) error {
	return workspace.UpdateConfig(fs, func(config workspace.WorkspaceConfig) (workspace.WorkspaceConfig, error) {
		removed := false
		updated := make([]workspace.WorkspaceLibrary, 0, len(config.Libraries))
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

func CollectLibraryDependents(config workspace.WorkspaceConfig, root string, name string, source string) []workspace.Target {
	var targets []workspace.Target
	for _, app := range config.Apps {
		for _, service := range app.Services {
			if hasLibraryDep(service.Deps, name, source) {
				targets = append(targets, workspace.Target{
					Kind: workspace.TargetService,
					App:  app.Name,
					Name: service.Name,
					Path: filepath.Join(root, "repos", "apps", app.Name, "services", service.Name),
				})
			}
		}
		for _, lib := range app.Libraries {
			if hasLibraryDep(lib.Deps, name, source) {
				targets = append(targets, workspace.Target{
					Kind: workspace.TargetAppLib,
					App:  app.Name,
					Name: lib.Name,
					Path: filepath.Join(root, "repos", "apps", app.Name, "libs", lib.Name),
				})
			}
		}
		for _, test := range app.Testing {
			if hasLibraryDep(test.Deps, name, source) {
				targets = append(targets, workspace.Target{
					Kind: workspace.TargetTest,
					App:  app.Name,
					Name: test.Name,
					Path: filepath.Join(root, "repos", "apps", app.Name, "testing", test.Name),
				})
			}
		}
	}
	for _, lib := range config.Libraries {
		if hasLibraryDep(lib.Deps, name, source) {
			targets = append(targets, workspace.Target{
				Kind: workspace.TargetGlobalLib,
				Name: lib.Name,
				Path: filepath.Join(root, "repos", "libs", lib.Name),
			})
		}
	}
	for _, project := range config.Projects {
		if hasLibraryDep(project.Deps, name, source) {
			targets = append(targets, workspace.Target{
				Kind: workspace.TargetProject,
				Name: project.Name,
				Path: filepath.Join(root, "repos", "projects", project.Name),
			})
		}
	}
	return targets
}

func RemoveLibraryDeps(config workspace.WorkspaceConfig, name string, source string) workspace.WorkspaceConfig {
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

func RemoveAppLibraryFromConfig(config workspace.WorkspaceConfig, appName string, name string) workspace.WorkspaceConfig {
	appIndex := workspace.FindAppIndex(config, appName)
	if appIndex == -1 {
		return config
	}
	libIndex := workspace.FindAppLibraryIndex(config.Apps[appIndex], name)
	if libIndex == -1 {
		return config
	}
	libs := config.Apps[appIndex].Libraries
	config.Apps[appIndex].Libraries = append(libs[:libIndex], libs[libIndex+1:]...)
	return config
}

func RemoveGlobalLibraryFromConfig(config workspace.WorkspaceConfig, name string) workspace.WorkspaceConfig {
	updated := make([]workspace.WorkspaceLibrary, 0, len(config.Libraries))
	for _, lib := range config.Libraries {
		if lib.Name == name {
			continue
		}
		updated = append(updated, lib)
	}
	config.Libraries = updated
	return config
}

func hasLibraryDep(depsList []workspace.WorkspaceDep, name string, source string) bool {
	for _, dep := range depsList {
		if dep.Lib == name && dep.From == source {
			return true
		}
	}
	return false
}

func filterLibraryDeps(depsList []workspace.WorkspaceDep, name string, source string) []workspace.WorkspaceDep {
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
