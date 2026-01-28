package projects

import (
	"fmt"
	"path/filepath"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/deps"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/workspace"
	"github.com/spf13/afero"
)

func FindProjectLanguage(fs afero.Fs, root string, name string) (bool, string, error) {
	projectPath := filepath.Join(root, "repos", "projects", name)
	return workspace.FindRepoLanguage(fs, projectPath)
}

func MakeProjectNode(config workspace.WorkspaceConfig, name string) (deps.Node, error) {
	project, ok, err := workspace.FindProjectByName(config, name)
	if err != nil {
		return deps.Node{}, err
	}
	if !ok {
		return deps.Node{}, fmt.Errorf("project not registered in workspace: %s", name)
	}
	return deps.Node{
		Kind:     deps.NodeProject,
		Name:     project.Name,
		Deps:     project.Deps,
	}, nil
}

// RemoveProjectFromWorkspace removes a project entry from ocean.workspace.json.
func RemoveProjectFromWorkspace(fs afero.Fs, name string) error {
	return workspace.UpdateConfig(fs, func(config workspace.WorkspaceConfig) (workspace.WorkspaceConfig, error) {
		removed := false
		updated := make([]workspace.WorkspaceProject, 0, len(config.Projects))
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
