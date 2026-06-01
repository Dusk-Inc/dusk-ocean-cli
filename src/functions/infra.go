package functions

import (
	"github.com/spf13/afero"
)

func FindInfraIndex(config WorkspaceConfig, name string) int {
	for i, entry := range config.Infrastructure {
		if entry.Name == name {
			return i
		}
	}
	return -1
}

func FindInfraByName(config WorkspaceConfig, name string) (WorkspaceInfra, bool) {
	idx := FindInfraIndex(config, name)
	if idx == -1 {
		return WorkspaceInfra{}, false
	}
	return config.Infrastructure[idx], true
}

func AddInfraToWorkspace(fs afero.Fs, name string) error {
	config, err := ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	if FindInfraIndex(config, name) != -1 {
		return nil
	}
	config.Infrastructure = append(config.Infrastructure, WorkspaceInfra{
		Name: name,
	})
	return WriteWorkspaceConfig(fs, config)
}

func RemoveInfraFromWorkspace(fs afero.Fs, name string) error {
	return UpdateConfig(fs, func(config WorkspaceConfig) (WorkspaceConfig, error) {
		removed := false
		updated := make([]WorkspaceInfra, 0, len(config.Infrastructure))
		for _, entry := range config.Infrastructure {
			if entry.Name == name {
				removed = true
				continue
			}
			updated = append(updated, entry)
		}
		if !removed {
			return config, nil
		}
		config.Infrastructure = updated
		return config, nil
	})
}

func InfraNames(config WorkspaceConfig) []string {
	names := make([]string, 0, len(config.Infrastructure))
	for _, entry := range config.Infrastructure {
		names = append(names, entry.Name)
	}
	return names
}
