package functions

import (
	"fmt"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

func FindTemplateIndex(config WorkspaceConfig, name string) int {
	for i, template := range config.Templates {
		if template.Name == name {
			return i
		}
	}
	return -1
}

func AddTemplateToWorkspace(fs afero.Fs, name string, kind string) error {
	if err := ValidateTemplateKind(kind); err != nil {
		return err
	}
	return UpdateConfig(fs, func(config WorkspaceConfig) (WorkspaceConfig, error) {
		if FindTemplateIndex(config, name) != -1 {
			return config, fmt.Errorf("template already registered: %s", name)
		}
		config.Templates = append(config.Templates, WorkspaceTemplate{
			Name: name,
			Kind: kind,
			Deps: []WorkspaceDep{},
		})
		return config, nil
	})
}

func RemoveTemplateFromWorkspace(fs afero.Fs, name string) error {
	return UpdateConfig(fs, func(config WorkspaceConfig) (WorkspaceConfig, error) {
		idx := FindTemplateIndex(config, name)
		if idx == -1 {
			return config, nil
		}
		config.Templates = append(config.Templates[:idx], config.Templates[idx+1:]...)
		return config, nil
	})
}

func FindTemplatesByKind(config WorkspaceConfig, kind string) []string {
	if kind == tokens.RepoKindApp {
		return nil
	}
	var names []string
	for _, template := range config.Templates {
		if template.Kind == kind {
			names = append(names, template.Name)
		}
	}
	return names
}

func FindTemplatesByKinds(config WorkspaceConfig, kinds ...string) []string {
	if len(kinds) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	var names []string
	for _, kind := range kinds {
		if kind == tokens.RepoKindApp {
			continue
		}
		for _, template := range config.Templates {
			if template.Kind != kind {
				continue
			}
			if _, ok := seen[template.Name]; ok {
				continue
			}
			seen[template.Name] = struct{}{}
			names = append(names, template.Name)
		}
	}
	return names
}

func ValidateTemplateDepsForTarget(root string, config WorkspaceConfig, templateName string, target Target) error {
	idx := FindTemplateIndex(config, templateName)
	if idx == -1 {

		return nil
	}
	for _, dep := range config.Templates[idx].Deps {
		dependency, err := resolveDependency(root, target, dep.Lib, config)
		if err != nil {
			return fmt.Errorf("template dep %s: %w", dep.Lib, err)
		}
		if err := validateInstallFlow(target, dependency); err != nil {
			return fmt.Errorf("template dep %s: %w", dep.Lib, err)
		}

		if dependency.kind == dependencyAppLib && target.App != dependency.app {
			payloadLookup := Target{Kind: TargetAppLib, App: dependency.app, Name: dependency.name}
			payloadScopes := FindTargetScopes(config, payloadLookup)
			if len(payloadScopes) == 0 {
				return fmt.Errorf("template dep %s: scope violation: %s has no declared scopes", dep.Lib, dep.Lib)
			}
		}
	}
	return nil
}

func PropagateTemplateDeps(cmd *cobra.Command, fs afero.Fs, templateName string, target Target) error {
	config, err := ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	idx := FindTemplateIndex(config, templateName)
	if idx == -1 {
		return nil
	}
	for _, dep := range config.Templates[idx].Deps {
		if err := WireLocalDependencyForTarget(cmd, fs, dep.Lib, target); err != nil {
			return fmt.Errorf("template dep %s: %w", dep.Lib, err)
		}
	}
	return nil
}
