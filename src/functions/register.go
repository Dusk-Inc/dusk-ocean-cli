package functions

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
	"github.com/spf13/afero"
)

// RegisterRepo brings a directory that already exists at one of the
// allowed workspace locations under Dusk Ocean management. It writes a
// starter ocean.config.json to mark the repo as registered, then adds an
// entry to ocean.workspace.json with the supplied (or default "None")
// remote URL.
//
// Behavior matrix for the target directory:
//
//	does not exist                  → not-found error
//	exists, no ocean.config.json    → write starter, register entry
//	exists, has ocean.config.json   → already-registered error
//
// register never clones or moves files; the developer is expected to have
// placed the repo at the deterministic path before invoking it.
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

	configPath := filepath.Join(relPath, "ocean.config.json")
	if _, err := fs.Stat(configPath); err == nil {
		return fmt.Errorf("repo is already registered at %s", relPath)
	} else if !os.IsNotExist(err) {
		return err
	}

	if remote == "" {
		remote = tokens.RemoteNone
	}

	starterType := kind
	if kind == tokens.RepoKindTemplate {
		// A template repo's ocean.config.json carries the kind it scaffolds
		// so the runtime can route it correctly via ListTemplatesByType.
		starterType = templateKind
	}
	if err := WriteStarterRepoConfig(fs, relPath, name, starterType); err != nil {
		return err
	}
	if err := registerEntryInWorkspace(fs, kind, name, app, remote, templateKind); err != nil {
		return err
	}
	fmt.Fprintf(out, "registered %s/%s at %s\n", kind, name, relPath)
	return nil
}

// registerEntryInWorkspace adds the new repo entry to ocean.workspace.json
// using the existing Add*ToWorkspace helpers, then sets the Remote field
// on the just-added entry via UpdateConfig. Splitting the work in two keeps
// the existing helper signatures unchanged.
func registerEntryInWorkspace(fs afero.Fs, kind string, name string, app string, remote string, templateKind string) error {
	switch kind {
	case tokens.RepoKindProject:
		if err := AddProjectToWorkspace(fs, name); err != nil {
			return err
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
	default:
		return fmt.Errorf("unknown --kind: %s", kind)
	}

	return UpdateConfig(fs, func(config WorkspaceConfig) (WorkspaceConfig, error) {
		setRemoteOnRepo(&config, kind, name, app, remote)
		return config, nil
	})
}

// setRemoteOnRepo finds the repo entry inside config (mutating it in place)
// and assigns the remote field. It is a no-op if the entry can't be found
// — that path is unreachable from registerEntryInWorkspace because the
// preceding Add*ToWorkspace step always creates the entry.
func setRemoteOnRepo(config *WorkspaceConfig, kind string, name string, app string, remote string) {
	switch kind {
	case tokens.RepoKindProject:
		idx := FindProjectIndex(*config, name)
		if idx == -1 {
			return
		}
		config.Projects[idx].Remote = remote
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
	}
}
