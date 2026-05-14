package functions

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

func RenameInWorkspaceConfig(config WorkspaceConfig, target Target, oldName string, newName string) (WorkspaceConfig, error) {
	switch target.Kind {
	case TargetApp:
		idx := FindAppIndex(config, oldName)
		if idx == -1 {
			return config, fmt.Errorf("app not registered in workspace: %s", oldName)
		}
		config.Apps[idx].Name = newName
		config = renameDepSourceInConfig(config, oldName, newName)

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

	default:
		return config, fmt.Errorf("unsupported target kind for rename")
	}

	return config, nil
}

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

func renameDepSourceInConfig(config WorkspaceConfig, oldFrom string, newFrom string) WorkspaceConfig {
	for i := range config.Apps {
		for j := range config.Apps[i].Libraries {
			config.Apps[i].Libraries[j].Deps = renameDepSourceInSlice(config.Apps[i].Libraries[j].Deps, oldFrom, newFrom)
		}
		for j := range config.Apps[i].Services {
			config.Apps[i].Services[j].Deps = renameDepSourceInSlice(config.Apps[i].Services[j].Deps, oldFrom, newFrom)
		}
		for j := range config.Apps[i].Testing {
			config.Apps[i].Testing[j].Deps = renameDepSourceInSlice(config.Apps[i].Testing[j].Deps, oldFrom, newFrom)
		}
	}
	return config
}

func renameDepSourceInSlice(deps []WorkspaceDep, oldFrom string, newFrom string) []WorkspaceDep {
	for i, dep := range deps {
		if dep.From == oldFrom {
			deps[i].From = newFrom
		}
	}
	return deps
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

func RenameHashFiles(fs afero.Fs, root string, target Target, oldName string, newName string) error {
	type hashPair struct {
		oldPath string
		newPath string
	}

	var pairs []hashPair

	switch target.Kind {
	case TargetApp:
		return renameAppHashDirs(fs, root, oldName, newName)
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

func renameAppHashDirs(fs afero.Fs, root string, oldName string, newName string) error {
	hashRoot := filepath.Join(root, ".ocean", "hashes")
	dirs := []string{
		filepath.Join(hashRoot, "build", "services"),
		filepath.Join(hashRoot, "build", "libs"),
		filepath.Join(hashRoot, "build", "tests"),
		filepath.Join(hashRoot, "check", "services"),
		filepath.Join(hashRoot, "check", "libs"),
		filepath.Join(hashRoot, "check", "tests"),
	}
	for _, dir := range dirs {
		oldPath := filepath.Join(dir, oldName)
		if _, err := fs.Stat(oldPath); err != nil {
			continue
		}
		newPath := filepath.Join(dir, newName)
		if err := fs.Rename(oldPath, newPath); err != nil {
			return err
		}
	}
	return nil
}

func ResolveTargetInApp(config WorkspaceConfig, root string, appName string, name string) (Target, error) {
	appIdx := FindAppIndex(config, appName)
	if appIdx == -1 {
		return Target{}, fmt.Errorf("app not found: %s", appName)
	}
	var matches []Target
	for _, svc := range config.Apps[appIdx].Services {
		if svc.Name == name {
			matches = append(matches, Target{
				Kind: TargetService,
				App:  appName,
				Name: name,
				Path: filepath.Join(root, "repos", "apps", appName, "services", name),
			})
		}
	}
	for _, lib := range config.Apps[appIdx].Libraries {
		if lib.Name == name {
			matches = append(matches, Target{
				Kind: TargetAppLib,
				App:  appName,
				Name: name,
				Path: filepath.Join(root, "repos", "apps", appName, "libs", name),
			})
		}
	}
	for _, test := range config.Apps[appIdx].Testing {
		if test.Name == name {
			matches = append(matches, Target{
				Kind: TargetTest,
				App:  appName,
				Name: name,
				Path: filepath.Join(root, "repos", "apps", appName, "testing", name),
			})
		}
	}
	if len(matches) == 0 {
		return Target{}, fmt.Errorf("target not found in app %s: %s", appName, name)
	}
	if len(matches) > 1 {
		return Target{}, fmt.Errorf("target name is ambiguous in app %s: %s", appName, name)
	}
	return matches[0], nil
}

func RenameRepo(cmd *cobra.Command, fs afero.Fs, oldName string, newName string, appName ...string) error {
	root, err := EnsureWorkspaceRoot(fs)
	if err != nil {
		return err
	}

	config, err := ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}

	var target Target
	inApp := ""
	if len(appName) > 0 {
		inApp = appName[0]
	}
	if inApp != "" {
		target, err = ResolveTargetInApp(config, root, inApp, oldName)
	} else {
		target, err = ResolveTargetByName(config, root, oldName)
	}
	if err != nil {
		return fmt.Errorf("target not found: %s", oldName)
	}

	if inApp != "" {
		if _, err := ResolveTargetInApp(config, root, inApp, newName); err == nil {
			return fmt.Errorf("name conflict: %s is already in use in app %s", newName, inApp)
		}
	} else {
		if _, err := ResolveTargetByName(config, root, newName); err == nil {
			return fmt.Errorf("name conflict: %s is already in use", newName)
		}
	}

	updatedConfig, err := RenameInWorkspaceConfig(config, target, oldName, newName)
	if err != nil {
		return err
	}

	if err := WriteWorkspaceConfig(fs, updatedConfig); err != nil {
		return err
	}

	repoConfig, err := ReadRepoConfig(fs, target.Path)
	if err != nil {
		repoConfig = RepoConfig{}
	}
	repoConfig.Name = newName
	if err := WriteRepoConfig(fs, target.Path, repoConfig); err != nil {
		return err
	}

	if err := RenameHashFiles(fs, root, target, oldName, newName); err != nil {
		return err
	}

	newDirPath := filepath.Join(filepath.Dir(target.Path), newName)
	if err := fs.Rename(target.Path, newDirPath); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "renamed %s to %s\n", oldName, newName)
	return nil
}
