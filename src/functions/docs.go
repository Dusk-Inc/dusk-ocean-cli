package functions

import (
	"github.com/spf13/afero"
)

func FindDocsIndex(config WorkspaceConfig, name string) int {
	for i, entry := range config.Docs {
		if entry.Name == name {
			return i
		}
	}
	return -1
}

func FindDocsByName(config WorkspaceConfig, name string) (WorkspaceDocs, bool) {
	idx := FindDocsIndex(config, name)
	if idx == -1 {
		return WorkspaceDocs{}, false
	}
	return config.Docs[idx], true
}

func AddDocsToWorkspace(fs afero.Fs, name string) error {
	config, err := ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	if FindDocsIndex(config, name) != -1 {
		return nil
	}
	config.Docs = append(config.Docs, WorkspaceDocs{
		Name: name,
	})
	return WriteWorkspaceConfig(fs, config)
}

func RemoveDocsFromWorkspace(fs afero.Fs, name string) error {
	return UpdateConfig(fs, func(config WorkspaceConfig) (WorkspaceConfig, error) {
		removed := false
		updated := make([]WorkspaceDocs, 0, len(config.Docs))
		for _, entry := range config.Docs {
			if entry.Name == name {
				removed = true
				continue
			}
			updated = append(updated, entry)
		}
		if !removed {
			return config, nil
		}
		config.Docs = updated
		return config, nil
	})
}

func DocsNames(config WorkspaceConfig) []string {
	names := make([]string, 0, len(config.Docs))
	for _, entry := range config.Docs {
		names = append(names, entry.Name)
	}
	return names
}
