package functions

import (
	"fmt"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// FindTemplateIndex returns the position of a registered template, or -1 when absent.
func FindTemplateIndex(config WorkspaceConfig, name string) int {
	for i, template := range config.Templates {
		if template.Name == name {
			return i
		}
	}
	return -1
}

// AddTemplateToWorkspace registers a template under the kind it scaffolds, rejecting an unknown kind or a duplicate name.
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

// RemoveTemplateFromWorkspace unregisters a template, no-op when it is absent.
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

// FindTemplatesByKind returns the names of every registered template that scaffolds the given kind.
func FindTemplatesByKind(config WorkspaceConfig, kind string) []string {
	var names []string
	for _, template := range config.Templates {
		if template.Kind == kind {
			names = append(names, template.Name)
		}
	}
	return names
}

// FindTemplatesByKinds returns the names of every registered template that scaffolds any of the given kinds, deduplicated.
func FindTemplatesByKinds(config WorkspaceConfig, kinds ...string) []string {
	if len(kinds) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	var names []string
	for _, kind := range kinds {
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

/*
ValidateAppTemplateDeps rejects an app template that declares deps. An app is
not itself an install target, so its nested units carry their own dependencies;
routing an app template's deps through the normal install flow would fail with
an opaque "unsupported install target".
*/
func ValidateAppTemplateDeps(config WorkspaceConfig, templateName string) error {
	idx := FindTemplateIndex(config, templateName)
	if idx == -1 {
		return nil
	}
	if len(config.Templates[idx].Deps) == 0 {
		return nil
	}
	return fmt.Errorf("app template %s declares deps; an app is not an install target, so its nested units must declare their own", templateName)
}

// ValidateTemplateDepsForTarget checks that every library a template declares resolves and may legally be installed into the target it is about to scaffold, so a bad scaffold fails before any file is copied.
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

// PropagateTemplateDeps wires each library a template declares into the freshly scaffolded target.
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
