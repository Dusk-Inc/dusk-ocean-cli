package functions

import (
	"fmt"
	"path/filepath"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
)

// ResolveRepoPath returns the deterministic on-disk location for a repo
// of the given kind. The path is workspace-root-relative; callers that
// need an absolute path should join the result with the workspace root.
//
// The mapping is:
//   - project              → repos/projects/<name>
//   - library + no app     → repos/libs/<name>
//   - library + app        → repos/apps/<app>/libs/<name>
//   - app                  → repos/apps/<name>
//   - service (app req'd)  → repos/apps/<app>/services/<name>
//   - template             → repos/templates/<name>
func ResolveRepoPath(kind string, name string, app string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("--name is required")
	}
	switch kind {
	case tokens.RepoKindProject:
		return filepath.Join("repos", "projects", name), nil
	case tokens.RepoKindLibrary:
		if app == "" {
			return filepath.Join("repos", "libs", name), nil
		}
		return filepath.Join("repos", "apps", app, "libs", name), nil
	case tokens.RepoKindApp:
		return filepath.Join("repos", "apps", name), nil
	case tokens.RepoKindService:
		if app == "" {
			return "", fmt.Errorf("--app is required when --kind is %s", kind)
		}
		return filepath.Join("repos", "apps", app, "services", name), nil
	case tokens.RepoKindTemplate:
		return filepath.Join("repos", "templates", name), nil
	}
	return "", fmt.Errorf("unknown --kind: %s", kind)
}

// ValidateRepoKindFlags rejects flag combinations that don't make sense for
// the chosen kind. Service requires --app; library accepts --app optionally
// (presence flips the entry from global to app-scoped); project, app, and
// template reject --app entirely.
func ValidateRepoKindFlags(kind string, app string) error {
	switch kind {
	case tokens.RepoKindProject, tokens.RepoKindApp, tokens.RepoKindTemplate:
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
	return fmt.Errorf("unknown --kind: %s (expected one of: %s, %s, %s, %s, %s)",
		kind,
		tokens.RepoKindProject, tokens.RepoKindLibrary, tokens.RepoKindApp, tokens.RepoKindService, tokens.RepoKindTemplate,
	)
}

// ValidateTemplateKind ensures --template-kind is one of the three allowed
// values. Apps are intentionally not template-able, so an explicit "app"
// value is rejected with a clear message rather than landing under the
// generic "unknown" branch.
func ValidateTemplateKind(value string) error {
	switch value {
	case tokens.TemplateKindService, tokens.TemplateKindLibrary, tokens.TemplateKindProject:
		return nil
	case "":
		return fmt.Errorf("--template-kind is required when --kind is %s", tokens.RepoKindTemplate)
	case tokens.RepoKindApp:
		return fmt.Errorf("--template-kind app is not supported: apps are not template-able")
	}
	return fmt.Errorf("unknown --template-kind: %s (expected one of: %s, %s, %s)",
		value,
		tokens.TemplateKindService, tokens.TemplateKindLibrary, tokens.TemplateKindProject,
	)
}

// ValidateTemplateKindFlag enforces that --template-kind is only supplied
// when --kind is template, and required when it is. Centralizing both rules
// here keeps adopt/register validation in one place.
func ValidateTemplateKindFlag(kind string, templateKind string) error {
	if kind != tokens.RepoKindTemplate {
		if templateKind != "" {
			return fmt.Errorf("--template-kind must not be set when --kind is %s", kind)
		}
		return nil
	}
	return ValidateTemplateKind(templateKind)
}
