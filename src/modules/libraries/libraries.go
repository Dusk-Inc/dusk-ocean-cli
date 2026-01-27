package libraries

import (
	"fmt"
	"path/filepath"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/apps"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/deps"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/workspace"
	"github.com/spf13/afero"
)

func FindGlobalLib(fs afero.Fs, root string, name string) (bool, string, error) {
	libPath := filepath.Join(root, "repos", "libs", name)
	info, err := fs.Stat(libPath)
	if err != nil {
		if afero.IsNotExist(err) {
			return false, "", nil
		}
		return false, "", err
	}
	if !info.IsDir() {
		return false, "", nil
	}
	config, err := workspace.ReadRepoConfig(fs, libPath)
	if err != nil {
		return true, "", nil
	}
	return true, config.Language, nil
}

func FindGlobalLibraryIndex(config workspace.WorkspaceConfig, libName string) int {
	for i, lib := range config.Libraries {
		if lib.Name == libName {
			return i
		}
	}
	return -1
}

func FindAppLibraryIndex(app workspace.WorkspaceApp, libName string) int {
	for i, lib := range app.Libraries {
		if lib.Name == libName {
			return i
		}
	}
	return -1
}

func FindAppLibraryByName(config workspace.WorkspaceConfig, appName string, name string) (workspace.WorkspaceLibrary, bool) {
	appIndex := apps.FindAppIndex(config, appName)
	if appIndex == -1 {
		return workspace.WorkspaceLibrary{}, false
	}
	for _, lib := range config.Apps[appIndex].Libraries {
		if lib.Name == name {
			return lib, true
		}
	}
	return workspace.WorkspaceLibrary{}, false
}

func FindGlobalLibraryByName(config workspace.WorkspaceConfig, name string) (workspace.WorkspaceLibrary, bool, error) {
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

func MakeAppLibNode(config workspace.WorkspaceConfig, appName string, name string) (deps.Node, error) {
	lib, ok := FindAppLibraryByName(config, appName, name)
	if !ok {
		return deps.Node{}, fmt.Errorf("library not registered in workspace: %s", name)
	}
	return deps.Node{
		Kind:     deps.NodeAppLib,
		App:      appName,
		Name:     name,
		Deps:     lib.Deps,
	}, nil
}

func MakeGlobalLibNode(config workspace.WorkspaceConfig, name string) (deps.Node, error) {
	lib, ok, err := FindGlobalLibraryByName(config, name)
	if err != nil {
		return deps.Node{}, err
	}
	if !ok {
		return deps.Node{}, fmt.Errorf("library not registered in workspace: %s", name)
	}
	return deps.Node{
		Kind:     deps.NodeGlobalLib,
		Name:     lib.Name,
		Deps:     lib.Deps,
	}, nil
}

func RemoveAppLibraryFromWorkspace(fs afero.Fs, appName string, name string) error {
	workspaceConfig, err := workspace.ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	appIndex := apps.FindAppIndex(workspaceConfig, appName)
	if appIndex == -1 {
		return nil
	}
	libIndex := FindAppLibraryIndex(workspaceConfig.Apps[appIndex], name)
	if libIndex == -1 {
		return nil
	}
	libs := workspaceConfig.Apps[appIndex].Libraries
	workspaceConfig.Apps[appIndex].Libraries = append(libs[:libIndex], libs[libIndex+1:]...)
	return workspace.WriteWorkspaceConfig(fs, workspaceConfig)
}

func RemoveGlobalLibraryFromWorkspace(fs afero.Fs, name string) error {
	workspaceConfig, err := workspace.ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	removed := false
	updated := make([]workspace.WorkspaceLibrary, 0, len(workspaceConfig.Libraries))
	for _, lib := range workspaceConfig.Libraries {
		if lib.Name == name {
			removed = true
			continue
		}
		updated = append(updated, lib)
	}
	if !removed {
		return nil
	}
	workspaceConfig.Libraries = updated
	return workspace.WriteWorkspaceConfig(fs, workspaceConfig)
}
