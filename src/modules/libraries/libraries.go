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
