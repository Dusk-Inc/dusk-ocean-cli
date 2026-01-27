package projects

import (
	"fmt"
	"path/filepath"
	"os"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/deps"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/workspace"
	"github.com/spf13/afero"
)

func FindProjectLanguage(fs afero.Fs, root string, name string) (bool, string, error) {
	projectPath := filepath.Join(root, "repos", "projects", name)
	info, err := fs.Stat(projectPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, "", nil
		}
		return false, "", err
	}
	if !info.IsDir() {
		return false, "", nil
	}
	config, err := workspace.ReadRepoConfig(fs, projectPath)
	if err != nil {
		return true, "", nil
	}
	return true, config.Language, nil
}

func FindProjectIndex(config workspace.WorkspaceConfig, projectName string) int {
	for i, project := range config.Projects {
		if project.Name == projectName {
			return i
		}
	}
	return -1
}

func FindProjectByName(config workspace.WorkspaceConfig, name string) (workspace.WorkspaceProject, bool, error) {
	var match *workspace.WorkspaceProject
	for i, project := range config.Projects {
		if project.Name != name {
			continue
		}
		if match != nil {
			return workspace.WorkspaceProject{}, false, fmt.Errorf("project name is ambiguous: %s", name)
		}
		match = &config.Projects[i]
	}
	if match == nil {
		return workspace.WorkspaceProject{}, false, nil
	}
	return *match, true, nil
}

func MakeProjectNode(config workspace.WorkspaceConfig, name string) (deps.Node, error) {
	project, ok, err := FindProjectByName(config, name)
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
	workspaceConfig, err := workspace.ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	removed := false
	updated := make([]workspace.WorkspaceProject, 0, len(workspaceConfig.Projects))
	for _, project := range workspaceConfig.Projects {
		if project.Name == name {
			removed = true
			continue
		}
		updated = append(updated, project)
	}
	if !removed {
		return nil
	}
	workspaceConfig.Projects = updated
	return workspace.WriteWorkspaceConfig(fs, workspaceConfig)
}
