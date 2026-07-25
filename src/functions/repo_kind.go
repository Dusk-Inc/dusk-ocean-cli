package functions

import (
	"fmt"
	"path/filepath"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
)

func ResolveRepoPath(kind string, name string, app string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("--name is required")
	}
	switch kind {
	case tokens.RepoKindProject:
		return filepath.Join("repos", tokens.RepoDirProjects, name), nil
	case tokens.RepoKindLibrary:
		if app == "" {
			return filepath.Join("repos", tokens.RepoDirLibs, name), nil
		}
		return filepath.Join("repos", tokens.RepoDirApps, app, tokens.RepoDirLibs, name), nil
	case tokens.RepoKindApp:
		return filepath.Join("repos", tokens.RepoDirApps, name), nil
	case tokens.RepoKindService:
		if app == "" {
			return "", fmt.Errorf("--app is required when --kind is %s", kind)
		}
		return filepath.Join("repos", tokens.RepoDirApps, app, tokens.RepoDirServices, name), nil
	case tokens.RepoKindTemplate:
		return filepath.Join("repos", tokens.RepoDirTemplates, name), nil
	case tokens.RepoKindInfra:
		return filepath.Join("repos", tokens.RepoDirInfra, name), nil
	case tokens.RepoKindDocs:
		return filepath.Join("repos", tokens.RepoDirDocs, name), nil
	}
	return "", fmt.Errorf("unknown --kind: %s", kind)
}

func ValidateRepoKindFlags(kind string, app string) error {
	switch kind {
	case tokens.RepoKindProject, tokens.RepoKindApp, tokens.RepoKindTemplate, tokens.RepoKindInfra, tokens.RepoKindDocs:
		if app != "" {
			return fmt.Errorf("--app must not be set when --kind is %s", kind)
		}
		return nil
	case tokens.RepoKindLibrary:
		return nil
	case tokens.RepoKindService:
		if app == "" {
			return fmt.Errorf("--app is required when --kind is %s", kind)
		}
		return nil
	}
	return fmt.Errorf("unknown --kind: %s (expected one of: %s, %s, %s, %s, %s, %s, %s)",
		kind,
		tokens.RepoKindProject, tokens.RepoKindLibrary, tokens.RepoKindApp, tokens.RepoKindService, tokens.RepoKindTemplate, tokens.RepoKindInfra, tokens.RepoKindDocs,
	)
}

func ValidateTemplateKind(value string) error {
	switch value {
	case tokens.TemplateKindService, tokens.TemplateKindLibrary, tokens.TemplateKindProject, tokens.TemplateKindInfra, tokens.TemplateKindDocs:
		return nil
	case "":
		return fmt.Errorf("--template-kind is required when --kind is %s", tokens.RepoKindTemplate)
	case tokens.RepoKindApp:
		return fmt.Errorf("--template-kind app is not supported: apps are not template-able")
	}
	return fmt.Errorf("unknown --template-kind: %s (expected one of: %s, %s, %s, %s, %s)",
		value,
		tokens.TemplateKindService, tokens.TemplateKindLibrary, tokens.TemplateKindProject, tokens.TemplateKindInfra, tokens.TemplateKindDocs,
	)
}

func ValidateTemplateKindFlag(kind string, templateKind string) error {
	if kind != tokens.RepoKindTemplate {
		if templateKind != "" {
			return fmt.Errorf("--template-kind must not be set when --kind is %s", kind)
		}
		return nil
	}
	return ValidateTemplateKind(templateKind)
}
