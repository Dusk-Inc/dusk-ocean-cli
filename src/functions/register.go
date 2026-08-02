package functions

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
	"github.com/spf13/afero"
)

// RegisterRepo records an existing on-disk repo in workspace config, discovering an app's nested sub-repos as well, and reports where it landed.
func RegisterRepo(fs afero.Fs, out io.Writer, kind string, name string, app string, remote string, templateKind string) error {
	if err := ValidateRepoKindFlags(kind, app); err != nil {
		return err
	}
	if err := ValidateTemplateKindFlag(kind, templateKind); err != nil {
		return err
	}
	relPath, err := ResolveRepoPath(kind, name, app)
	if err != nil {
		return err
	}

	info, err := fs.Stat(relPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no directory at %s", relPath)
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s exists and is not a directory", relPath)
	}

	workspaceConfig, err := ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	if IsRegisteredInWorkspace(workspaceConfig, kind, name, app) {
		return fmt.Errorf("repo is already registered in workspace: %s/%s", kind, name)
	}

	if remote == "" {
		remote = tokens.RemoteNone
	}

	configPath := filepath.Join(relPath, "ocean.config.json")
	if _, err := fs.Stat(configPath); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		starterType := kind
		if kind == tokens.RepoKindTemplate {

			starterType = templateKind
		}
		if err := WriteStarterRepoConfig(fs, relPath, name, starterType); err != nil {
			return err
		}
	}

	if err := registerEntryInWorkspace(fs, kind, name, app, remote, templateKind); err != nil {
		return err
	}

	if kind == tokens.RepoKindApp {
		if err := RegisterDiscoveredAppSubRepos(fs, out, name); err != nil {
			return err
		}
	}

	fmt.Fprintf(out, "registered %s/%s at %s\n", kind, name, relPath)
	return nil
}

// IsRegisteredInWorkspace reports whether workspace config already carries an entry for the given repo kind, name, and optional parent app.
func IsRegisteredInWorkspace(config WorkspaceConfig, kind string, name string, app string) bool {
	switch kind {
	case tokens.RepoKindProject:
		if app == "" {
			return FindProjectIndex(config, name) != -1
		}
		appIdx := FindAppIndex(config, app)
		if appIdx == -1 {
			return false
		}
		return FindAppProjectIndex(config.Apps[appIdx], name) != -1
	case tokens.RepoKindLibrary:
		if app == "" {
			return FindGlobalLibraryIndex(config, name) != -1
		}
		appIdx := FindAppIndex(config, app)
		if appIdx == -1 {
			return false
		}
		return FindAppLibraryIndex(config.Apps[appIdx], name) != -1
	case tokens.RepoKindApp:
		return FindAppIndex(config, name) != -1
	case tokens.RepoKindService:
		appIdx := FindAppIndex(config, app)
		if appIdx == -1 {
			return false
		}
		return FindServiceIndex(config.Apps[appIdx], name) != -1
	case tokens.RepoKindTemplate:
		return FindTemplateIndex(config, name) != -1
	case tokens.RepoKindInfra:
		return FindInfraIndex(config, name) != -1
	case tokens.RepoKindDocs:
		return FindDocsIndex(config, name) != -1
	}
	return false
}

// registerEntryInWorkspace appends the workspace-config entry for one repo kind and records its remote.
func registerEntryInWorkspace(fs afero.Fs, kind string, name string, app string, remote string, templateKind string) error {
	switch kind {
	case tokens.RepoKindProject:
		if app == "" {
			if err := AddProjectToWorkspace(fs, name); err != nil {
				return err
			}
		} else {
			if err := AddAppProjectToWorkspace(fs, app, name); err != nil {
				return err
			}
		}
	case tokens.RepoKindLibrary:
		if app == "" {
			if err := AddGlobalLibraryToWorkspace(fs, name); err != nil {
				return err
			}
		} else {
			if err := AddAppLibraryToWorkspace(fs, app, name); err != nil {
				return err
			}
		}
	case tokens.RepoKindApp:
		if err := addAppToWorkspace(fs, name); err != nil {
			return err
		}
	case tokens.RepoKindService:
		port, err := NextServicePort(fs, app)
		if err != nil {
			return err
		}
		image := DefaultServiceImage(app, name)
		if err := AddServiceToWorkspace(fs, app, name, port, image, "", ""); err != nil {
			return err
		}
	case tokens.RepoKindTemplate:
		if err := AddTemplateToWorkspace(fs, name, templateKind); err != nil {
			return err
		}
	case tokens.RepoKindInfra:
		if err := AddInfraToWorkspace(fs, name); err != nil {
			return err
		}
	case tokens.RepoKindDocs:
		if err := AddDocsToWorkspace(fs, name); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown --kind: %s", kind)
	}

	return UpdateConfig(fs, func(config WorkspaceConfig) (WorkspaceConfig, error) {
		setRemoteOnRepo(&config, kind, name, app, remote)
		return config, nil
	})
}

// setRemoteOnRepo writes a remote onto the registered entry for one repo kind, no-op when the entry is absent.
func setRemoteOnRepo(config *WorkspaceConfig, kind string, name string, app string, remote string) {
	switch kind {
	case tokens.RepoKindProject:
		if app == "" {
			idx := FindProjectIndex(*config, name)
			if idx == -1 {
				return
			}
			config.Projects[idx].Remote = remote
			return
		}
		appIdx := FindAppIndex(*config, app)
		if appIdx == -1 {
			return
		}
		projectIdx := FindAppProjectIndex(config.Apps[appIdx], name)
		if projectIdx == -1 {
			return
		}
		config.Apps[appIdx].Projects[projectIdx].Remote = remote
	case tokens.RepoKindLibrary:
		if app == "" {
			idx := FindGlobalLibraryIndex(*config, name)
			if idx == -1 {
				return
			}
			config.Libraries[idx].Remote = remote
			return
		}
		appIdx := FindAppIndex(*config, app)
		if appIdx == -1 {
			return
		}
		libIdx := FindAppLibraryIndex(config.Apps[appIdx], name)
		if libIdx == -1 {
			return
		}
		config.Apps[appIdx].Libraries[libIdx].Remote = remote
	case tokens.RepoKindApp:
		idx := FindAppIndex(*config, name)
		if idx == -1 {
			return
		}
		config.Apps[idx].Remote = remote
	case tokens.RepoKindService:
		appIdx := FindAppIndex(*config, app)
		if appIdx == -1 {
			return
		}
		svcIdx := FindServiceIndex(config.Apps[appIdx], name)
		if svcIdx == -1 {
			return
		}
		config.Apps[appIdx].Services[svcIdx].Remote = remote
	case tokens.RepoKindTemplate:
		idx := FindTemplateIndex(*config, name)
		if idx == -1 {
			return
		}
		config.Templates[idx].Remote = remote
	case tokens.RepoKindInfra:
		idx := FindInfraIndex(*config, name)
		if idx == -1 {
			return
		}
		config.Infrastructure[idx].Remote = remote
	case tokens.RepoKindDocs:
		idx := FindDocsIndex(*config, name)
		if idx == -1 {
			return
		}
		config.Docs[idx].Remote = remote
	}
}
