package functions

import (
	"fmt"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// FindTemplateIndex returns the index of the named template in the
// workspace config, or -1 if it is not registered.
func FindTemplateIndex(config WorkspaceConfig, name string) int {
	for i, template := range config.Templates {
		if template.Name == name {
			return i
		}
	}
	return -1
}

// AddTemplateToWorkspace appends a new WorkspaceTemplate entry. Apps are
// not template-able, so kinds other than service|library|project are
// rejected up-front. The Deps slice starts empty so the JSON shape stays
// stable.
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

// RemoveTemplateFromWorkspace removes a template entry by name. A no-op
// when the template is not registered.
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

// FindTemplatesByKind returns the names of every registered template whose
// Kind matches the requested value. Apps are intentionally excluded — even
// if a stale entry exists, asking for kind "app" yields nothing.
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

// ValidateTemplateDepsForTarget pre-validates every dep declared on a
// template against a hypothetical target, BEFORE any files are copied.
// The hypothetical target need not exist in workspace config yet — the
// check only walks the template's deps and asks "would each one be a
// legal addition for a repo of this kind/app?". Used by template-driven
// scaffolds to abort cleanly when a propagation would fail.
func ValidateTemplateDepsForTarget(root string, config WorkspaceConfig, templateName string, target Target) error {
	idx := FindTemplateIndex(config, templateName)
	if idx == -1 {
		// Filesystem-only templates have no registered deps to propagate.
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
		// REQ 7.4: cross-app app-lib deps need a shared scope.
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

// PropagateTemplateDeps wires each dep declared on a template into a
// freshly-scaffolded repo. The freshly-scaffolded repo is expected to be
// already registered in workspace config — callers should run this AFTER
// `Add*ToWorkspace` and AFTER `ValidateTemplateDepsForTarget` succeeds.
//
// Each dep is wired by delegating to WireLocalDependency, so the same
// scope/cycle/flow checks and `add` task execution apply as if the user
// had run `dusk-ocean add` by hand. Templates with no registered entry
// (filesystem-only templates) are a no-op.
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
		if err := WireLocalDependency(cmd, fs, dep.Lib, target.Name); err != nil {
			return fmt.Errorf("template dep %s: %w", dep.Lib, err)
		}
	}
	return nil
}
