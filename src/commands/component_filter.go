package cmd

import (
	"strings"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/tree"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/workspace"
	"github.com/spf13/afero"
)

func filterComponentsByLanguage(fs afero.Fs, components []tree.Component, language string) ([]tree.Component, error) {
	if strings.TrimSpace(language) == "" {
		return components, nil
	}
	filtered := make([]tree.Component, 0, len(components))
	for _, component := range components {
		config, err := workspace.ReadRepoConfig(fs, component.Path)
		if err != nil {
			return nil, err
		}
		if strings.EqualFold(config.Language, language) {
			filtered = append(filtered, component)
		}
	}
	return filtered, nil
}
