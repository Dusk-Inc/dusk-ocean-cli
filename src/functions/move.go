package functions

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

type MoveLibraryOptions struct {
	Library    string
	FromApp    string
	ToApp      string
	FromGlobal bool
	ToGlobal   bool
}

func MoveInWorkspaceConfig(config WorkspaceConfig, opts MoveLibraryOptions) (WorkspaceConfig, error) {
	oldFrom := opts.FromApp
	if opts.FromGlobal {
		oldFrom = "global"
	}
	newFrom := opts.ToApp
	if opts.ToGlobal {
		newFrom = "global"
	}

	var lib WorkspaceLibrary
	if opts.FromGlobal {
		idx := FindGlobalLibraryIndex(config, opts.Library)
		if idx == -1 {
			return config, fmt.Errorf("library not registered as global: %s", opts.Library)
		}
		lib = config.Libraries[idx]
		config.Libraries = append(config.Libraries[:idx], config.Libraries[idx+1:]...)
	} else {
		appIdx := FindAppIndex(config, opts.FromApp)
		if appIdx == -1 {
			return config, fmt.Errorf("source app not found: %s", opts.FromApp)
		}
		libIdx := FindAppLibraryIndex(config.Apps[appIdx], opts.Library)
		if libIdx == -1 {
			return config, fmt.Errorf("library not registered in app %s: %s", opts.FromApp, opts.Library)
		}
		lib = config.Apps[appIdx].Libraries[libIdx]
		config.Apps[appIdx].Libraries = append(
			config.Apps[appIdx].Libraries[:libIdx],
			config.Apps[appIdx].Libraries[libIdx+1:]...,
		)
	}

	if opts.ToGlobal {
		if FindGlobalLibraryIndex(config, opts.Library) != -1 {
			return config, fmt.Errorf("name conflict: %s already exists as a global library", opts.Library)
		}
		config.Libraries = append(config.Libraries, lib)
	} else {
		appIdx := FindAppIndex(config, opts.ToApp)
		if appIdx == -1 {
			return config, fmt.Errorf("destination app not found: %s", opts.ToApp)
		}
		if FindAppLibraryIndex(config.Apps[appIdx], opts.Library) != -1 {
			return config, fmt.Errorf("name conflict: %s already exists in app %s", opts.Library, opts.ToApp)
		}
		config.Apps[appIdx].Libraries = append(config.Apps[appIdx].Libraries, lib)
	}

	config = renameDepsInConfig(config, opts.Library, oldFrom, opts.Library, newFrom)

	return config, nil
}

func FindMoveScopeWarnings(config WorkspaceConfig, opts MoveLibraryOptions) []string {

	if opts.ToGlobal {
		return nil
	}

	destApp := opts.ToApp
	libScopes := findLibScopesInConfig(config, opts.Library, destApp)

	var warnings []string
	checkDep := func(ownerApp, ownerKind, ownerName string, dep WorkspaceDep) {
		if dep.Lib != opts.Library {
			return
		}
		if ownerApp == destApp {
			return
		}
		depTarget := Target{Kind: TargetAppLib, App: destApp, Name: opts.Library}
		ownerScopes := findOwnerScopes(config, ownerApp, ownerKind, ownerName)
		_ = depTarget
		if !HasCommonScope(libScopes, ownerScopes) {
			warnings = append(warnings, fmt.Sprintf(
				"warning: moving %s to app %s may break cross-app dependency from %s %s/%s (no shared scope)",
				opts.Library, destApp, ownerKind, ownerApp, ownerName,
			))
		}
	}

	for _, app := range config.Apps {
		for _, svc := range app.Services {
			for _, dep := range svc.Deps {
				checkDep(app.Name, "service", svc.Name, dep)
			}
		}
		for _, lib := range app.Libraries {
			for _, dep := range lib.Deps {
				checkDep(app.Name, "library", lib.Name, dep)
			}
		}
		for _, test := range app.Testing {
			for _, dep := range test.Deps {
				checkDep(app.Name, "test", test.Name, dep)
			}
		}
	}
	for _, proj := range config.Projects {
		for _, dep := range proj.Deps {
			if dep.Lib != opts.Library {
				continue
			}
			if !HasCommonScope(libScopes, proj.Scopes) {
				warnings = append(warnings, fmt.Sprintf(
					"warning: moving %s to app %s may break dependency from project %s (no shared scope)",
					opts.Library, destApp, proj.Name,
				))
			}
		}
	}

	return warnings
}

func findLibScopesInConfig(config WorkspaceConfig, libName, appName string) []string {
	appIdx := FindAppIndex(config, appName)
	if appIdx == -1 {
		return nil
	}
	libIdx := FindAppLibraryIndex(config.Apps[appIdx], libName)
	if libIdx == -1 {
		return nil
	}
	return config.Apps[appIdx].Libraries[libIdx].Scopes
}

func findOwnerScopes(config WorkspaceConfig, appName, kind, name string) []string {
	appIdx := FindAppIndex(config, appName)
	if appIdx == -1 {
		return nil
	}
	app := config.Apps[appIdx]
	switch kind {
	case "service":
		idx := FindServiceIndex(app, name)
		if idx == -1 {
			return nil
		}
		return app.Services[idx].Scopes
	case "library":
		idx := FindAppLibraryIndex(app, name)
		if idx == -1 {
			return nil
		}
		return app.Libraries[idx].Scopes
	case "test":
		idx := FindAppTestIndex(app, name)
		if idx == -1 {
			return nil
		}
		return app.Testing[idx].Scopes
	}
	return nil
}

func MoveHashFiles(fs afero.Fs, root string, opts MoveLibraryOptions) error {
	oldParts := hashPartsForLib(opts.Library, opts.FromApp, opts.FromGlobal)
	newParts := hashPartsForLib(opts.Library, opts.ToApp, opts.ToGlobal)

	pairs := []struct{ oldPath, newPath string }{
		{MakeHashPath(root, "libs", oldParts...), MakeHashPath(root, "libs", newParts...)},
		{MakeCheckHashPath(root, "libs", oldParts...), MakeCheckHashPath(root, "libs", newParts...)},
	}

	for _, pair := range pairs {
		if err := renameFileIfExists(fs, pair.oldPath, pair.newPath); err != nil {
			return err
		}
	}
	return nil
}

func hashPartsForLib(name, app string, isGlobal bool) []string {
	if isGlobal {
		return []string{"global", name}
	}
	return []string{app, name}
}

func MoveLibrary(cmd *cobra.Command, fs afero.Fs, opts MoveLibraryOptions) error {
	root, err := EnsureWorkspaceRoot(fs)
	if err != nil {
		return err
	}

	config, err := ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}

	oldPath := libDirPath(root, opts.Library, opts.FromApp, opts.FromGlobal)
	newPath := libDirPath(root, opts.Library, opts.ToApp, opts.ToGlobal)

	updatedConfig, err := MoveInWorkspaceConfig(config, opts)
	if err != nil {
		return err
	}

	for _, warning := range FindMoveScopeWarnings(updatedConfig, opts) {
		fmt.Fprintln(cmd.OutOrStdout(), warning)
	}

	if err := WriteWorkspaceConfig(fs, updatedConfig); err != nil {
		return err
	}

	if err := MoveHashFiles(fs, root, opts); err != nil {
		return err
	}

	if err := fs.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		return err
	}
	if err := fs.Rename(oldPath, newPath); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "moved library %s\n", opts.Library)
	return nil
}

func libDirPath(root, name, app string, isGlobal bool) string {
	if isGlobal {
		return filepath.Join(root, "repos", "libs", name)
	}
	return filepath.Join(root, "repos", "apps", app, "libs", name)
}
