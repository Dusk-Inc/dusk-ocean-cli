package functions

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/afero"
)

func FindProjectLanguage(fs afero.Fs, root string, name string) (bool, string, error) {
	projectPath := filepath.Join(root, "repos", "projects", name)
	return FindRepoLanguage(fs, projectPath)
}

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

// RemoveProjectFromWorkspace removes a project entry from ocean.workspace.json.
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
