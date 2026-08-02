package functions

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/afero"
)

// FindProjectLanguage reports the detected language of a global project repo.
func FindProjectLanguage(fs afero.Fs, root string, name string) (bool, string, error) {
	projectPath := filepath.Join(root, "repos", "projects", name)
	return FindRepoLanguage(fs, projectPath)
}

// MakeProjectNode builds the dependency-graph node for a registered global project.
func MakeProjectNode(config WorkspaceConfig, name string) (Node, error) {
	project, ok, err := FindProjectByName(config, name)
	if err != nil {
		return Node{}, err
	}
	if !ok {
		return Node{}, fmt.Errorf("project not registered in workspace: %s", name)
	}
	return Node{
		Kind: NodeProject,
		Name: project.Name,
		Deps: project.Deps,
	}, nil
}

// MakeAppProjectNode builds the dependency-graph node for a registered app-scoped project.
func MakeAppProjectNode(config WorkspaceConfig, appName string, name string) (Node, error) {
	project, ok := FindAppProjectByName(config, appName, name)
	if !ok {
		return Node{}, fmt.Errorf("project not registered in workspace: %s", name)
	}
	return Node{
		Kind: NodeAppProject,
		App:  appName,
		Name: project.Name,
		Deps: project.Deps,
	}, nil
}

// FindAppProjectLanguage reports the detected language of an app-scoped project repo.
func FindAppProjectLanguage(fs afero.Fs, root string, appName string, name string) (bool, string, error) {
	projectPath := filepath.Join(root, "repos", "apps", appName, "projects", name)
	return FindRepoLanguage(fs, projectPath)
}

// AddProjectToWorkspace registers a global project, no-op when it is already present.
func AddProjectToWorkspace(fs afero.Fs, name string) error {
	config, err := ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	if FindProjectIndex(config, name) != -1 {
		return nil
	}
	config.Projects = append(config.Projects, WorkspaceProject{
		Name: name,
		Deps: []WorkspaceDep{},
	})
	return WriteWorkspaceConfig(fs, config)
}

// AddAppProjectToWorkspace registers a project under an app, creating the app entry when absent.
func AddAppProjectToWorkspace(fs afero.Fs, appName string, name string) error {
	config, err := ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	appIndex := FindAppIndex(config, appName)
	if appIndex == -1 {
		config.Apps = append(config.Apps, WorkspaceApp{
			Name:      appName,
			Services:  []WorkspaceService{},
			Libraries: []WorkspaceLibrary{},
			Projects: []WorkspaceProject{
				{
					Name: name,
					Deps: []WorkspaceDep{},
				},
			},
			Testing: []WorkspaceTest{},
		})
		return WriteWorkspaceConfig(fs, config)
	}
	if FindAppProjectIndex(config.Apps[appIndex], name) != -1 {
		return nil
	}
	config.Apps[appIndex].Projects = append(config.Apps[appIndex].Projects, WorkspaceProject{
		Name: name,
		Deps: []WorkspaceDep{},
	})
	return WriteWorkspaceConfig(fs, config)
}

// RemoveAppProjectFromWorkspace unregisters a project from its app, no-op when absent.
func RemoveAppProjectFromWorkspace(fs afero.Fs, appName string, name string) error {
	return UpdateConfig(fs, func(config WorkspaceConfig) (WorkspaceConfig, error) {
		appIndex := FindAppIndex(config, appName)
		if appIndex == -1 {
			return config, nil
		}
		projectIndex := FindAppProjectIndex(config.Apps[appIndex], name)
		if projectIndex == -1 {
			return config, nil
		}
		projects := config.Apps[appIndex].Projects
		config.Apps[appIndex].Projects = append(projects[:projectIndex], projects[projectIndex+1:]...)
		return config, nil
	})
}

// RemoveProjectFromWorkspace unregisters a global project, no-op when it is absent.
func RemoveProjectFromWorkspace(fs afero.Fs, name string) error {
	return UpdateConfig(fs, func(config WorkspaceConfig) (WorkspaceConfig, error) {
		removed := false
		updated := make([]WorkspaceProject, 0, len(config.Projects))
		for _, project := range config.Projects {
			if project.Name == name {
				removed = true
				continue
			}
			updated = append(updated, project)
		}
		if !removed {
			return config, nil
		}
		config.Projects = updated
		return config, nil
	})
}
