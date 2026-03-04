package functions

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// RenameInWorkspaceConfig updates the target's Name field and all dep references in workspace config.
// It is a pure function and does not perform any disk I/O.
func RenameInWorkspaceConfig(config WorkspaceConfig, target Target, oldName string, newName string) (WorkspaceConfig, error) {
	switch target.Kind {
	case TargetGlobalLib:
		idx := FindGlobalLibraryIndex(config, oldName)
		if idx == -1 {
			return config, fmt.Errorf("library not registered in workspace: %s", oldName)
		}
		config.Libraries[idx].Name = newName
		config = renameDepsInConfig(config, oldName, "global", newName, "global")

	case TargetProject:
		idx := FindProjectIndex(config, oldName)
		if idx == -1 {
			return config, fmt.Errorf("project not registered in workspace: %s", oldName)
		}
		config.Projects[idx].Name = newName
		config = renameDepsInConfig(config, oldName, "project", newName, "project")

	case TargetAppLib:
		appIdx := FindAppIndex(config, target.App)
		if appIdx == -1 {
			return config, fmt.Errorf("app not registered in workspace: %s", target.App)
		}
		libIdx := FindAppLibraryIndex(config.Apps[appIdx], oldName)
		if libIdx == -1 {
			return config, fmt.Errorf("library not registered in workspace: %s", oldName)
		}
		config.Apps[appIdx].Libraries[libIdx].Name = newName
		config = renameDepsInConfig(config, oldName, target.App, newName, target.App)

	case TargetService:
		appIdx := FindAppIndex(config, target.App)
		if appIdx == -1 {
			return config, fmt.Errorf("app not registered in workspace: %s", target.App)
		}
		svcIdx := FindServiceIndex(config.Apps[appIdx], oldName)
		if svcIdx == -1 {
			return config, fmt.Errorf("service not registered in workspace: %s", oldName)
		}
		config.Apps[appIdx].Services[svcIdx].Name = newName
		// Services are not depended upon, so no dep-reference update needed.

	case TargetTest:
		appIdx := FindAppIndex(config, target.App)
		if appIdx == -1 {
			return config, fmt.Errorf("app not registered in workspace: %s", target.App)
		}
		testIdx := FindAppTestIndex(config.Apps[appIdx], oldName)
		if testIdx == -1 {
			return config, fmt.Errorf("test not registered in workspace: %s", oldName)
		}
		config.Apps[appIdx].Testing[testIdx].Name = newName
		// Tests are not depended upon, so no dep-reference update needed.

	default:
		return config, fmt.Errorf("unsupported target kind for rename")
	}

	return config, nil
}

// renameDepsInConfig replaces all WorkspaceDep entries matching oldLib/oldFrom
// with newLib/newFrom throughout the entire workspace config.
func renameDepsInConfig(config WorkspaceConfig, oldLib string, oldFrom string, newLib string, newFrom string) WorkspaceConfig {
	for i := range config.Libraries {
		config.Libraries[i].Deps = renameDepsInSlice(config.Libraries[i].Deps, oldLib, oldFrom, newLib, newFrom)
	}
	for i := range config.Projects {
		config.Projects[i].Deps = renameDepsInSlice(config.Projects[i].Deps, oldLib, oldFrom, newLib, newFrom)
	}
	for i := range config.Apps {
		for j := range config.Apps[i].Libraries {
			config.Apps[i].Libraries[j].Deps = renameDepsInSlice(config.Apps[i].Libraries[j].Deps, oldLib, oldFrom, newLib, newFrom)
		}
		for j := range config.Apps[i].Services {
			config.Apps[i].Services[j].Deps = renameDepsInSlice(config.Apps[i].Services[j].Deps, oldLib, oldFrom, newLib, newFrom)
		}
		for j := range config.Apps[i].Testing {
			config.Apps[i].Testing[j].Deps = renameDepsInSlice(config.Apps[i].Testing[j].Deps, oldLib, oldFrom, newLib, newFrom)
		}
	}
	return config
}

func renameDepsInSlice(deps []WorkspaceDep, oldLib string, oldFrom string, newLib string, newFrom string) []WorkspaceDep {
	for i, dep := range deps {
		if dep.Lib == oldLib && dep.From == oldFrom {
			deps[i].Lib = newLib
			deps[i].From = newFrom
		}
	}
	return deps
}

// RenameHashFiles moves the build and check hash files for the renamed target.
// Missing hash files are silently skipped.
func RenameHashFiles(fs afero.Fs, root string, target Target, oldName string, newName string) error {
	type hashPair struct {
		oldPath string
		newPath string
	}

	var pairs []hashPair

	switch target.Kind {
	case TargetGlobalLib:
		pairs = []hashPair{
			{MakeHashPath(root, "libs", "global", oldName), MakeHashPath(root, "libs", "global", newName)},
			{MakeCheckHashPath(root, "libs", "global", oldName), MakeCheckHashPath(root, "libs", "global", newName)},
		}
	case TargetAppLib:
		pairs = []hashPair{
			{MakeHashPath(root, "libs", target.App, oldName), MakeHashPath(root, "libs", target.App, newName)},
			{MakeCheckHashPath(root, "libs", target.App, oldName), MakeCheckHashPath(root, "libs", target.App, newName)},
		}
	case TargetService:
		pairs = []hashPair{
			{MakeHashPath(root, "services", target.App, oldName), MakeHashPath(root, "services", target.App, newName)},
			{MakeCheckHashPath(root, "services", target.App, oldName), MakeCheckHashPath(root, "services", target.App, newName)},
		}
	case TargetProject:
		pairs = []hashPair{
			{MakeHashPath(root, "projects", oldName), MakeHashPath(root, "projects", newName)},
			{MakeCheckHashPath(root, "projects", oldName), MakeCheckHashPath(root, "projects", newName)},
		}
	case TargetTest:
		pairs = []hashPair{
			{MakeHashPath(root, "tests", target.App, oldName), MakeHashPath(root, "tests", target.App, newName)},
			{MakeCheckHashPath(root, "tests", target.App, oldName), MakeCheckHashPath(root, "tests", target.App, newName)},
		}
	default:
		return fmt.Errorf("unsupported target kind for hash rename")
	}

	for _, pair := range pairs {
		if err := renameFileIfExists(fs, pair.oldPath, pair.newPath); err != nil {
			return err
		}
	}
	return nil
}

// renameFileIfExists moves oldPath to newPath, silently skipping if oldPath does not exist.
func renameFileIfExists(fs afero.Fs, oldPath string, newPath string) error {
	content, found, err := ReadHashFile(fs, oldPath)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if err := WriteHashFile(fs, newPath, content); err != nil {
		return err
	}
	return fs.Remove(oldPath)
}

// RenameRepo renames a repository and propagates changes throughout the workspace (REQ 8.1/8.2/8.3).
func RenameRepo(cmd *cobra.Command, fs afero.Fs, oldName string, newName string) error {
	root, err := EnsureWorkspaceRoot(fs)
	if err != nil {
		return err
	}

	config, err := ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}

	// REQ 8.3: reject if the target does not exist.
	target, err := ResolveTargetByName(config, root, oldName)
	if err != nil {
		return fmt.Errorf("target not found: %s", oldName)
	}

	// REQ 8.2: reject if new name is already in use.
	if _, err := ResolveTargetByName(config, root, newName); err == nil {
		return fmt.Errorf("name conflict: %s is already in use", newName)
	}

	// Update workspace config (pure — no disk I/O).
	updatedConfig, err := RenameInWorkspaceConfig(config, target, oldName, newName)
	if err != nil {
		return err
	}

	if err := WriteWorkspaceConfig(fs, updatedConfig); err != nil {
		return err
	}

	// Update ocean.config.json inside the repo.
	repoConfig, err := ReadRepoConfig(fs, target.Path)
	if err != nil {
		repoConfig = RepoConfig{}
	}
	repoConfig.Name = newName
	if err := WriteRepoConfig(fs, target.Path, repoConfig); err != nil {
		return err
	}

	// Rename hash files.
	if err := RenameHashFiles(fs, root, target, oldName, newName); err != nil {
		return err
	}

	// Rename the directory on disk last, so target.Path remains valid for all prior steps.
	newDirPath := filepath.Join(filepath.Dir(target.Path), newName)
	if err := fs.Rename(target.Path, newDirPath); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "renamed %s to %s\n", oldName, newName)
	return nil
}
