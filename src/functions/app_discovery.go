package functions

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/afero"
)

const (
	AppSubRepoKindService = "service"
	AppSubRepoKindLibrary = "library"
	AppSubRepoKindProject = "project"
	AppSubRepoKindTest    = "test"
)

type DiscoveredAppSubRepo struct {
	Kind string
	Name string
	Path string
}

var appSubRepoDirs = []struct {
	subdir string
	kind   string
}{
	{"services", AppSubRepoKindService},
	{"libs", AppSubRepoKindLibrary},
	{"projects", AppSubRepoKindProject},
	{"testing", AppSubRepoKindTest},
}

// DiscoverAppSubRepos lists the services, libraries, projects, and tests an app directory holds, each identified by its own ocean.config.json.
func DiscoverAppSubRepos(fs afero.Fs, appName string) ([]DiscoveredAppSubRepo, error) {
	appRoot := filepath.Join("repos", "apps", appName)
	var found []DiscoveredAppSubRepo

	for _, entry := range appSubRepoDirs {
		subRoot := filepath.Join(appRoot, entry.subdir)
		infos, err := afero.ReadDir(fs, subRoot)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		sort.Slice(infos, func(i, j int) bool {
			return infos[i].Name() < infos[j].Name()
		})

		for _, info := range infos {
			if !info.IsDir() {
				continue
			}
			childPath := filepath.Join(subRoot, info.Name())
			configPath := filepath.Join(childPath, "ocean.config.json")
			if _, err := fs.Stat(configPath); err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, err
			}
			found = append(found, DiscoveredAppSubRepo{
				Kind: entry.kind,
				Name: info.Name(),
				Path: childPath,
			})
		}
	}
	return found, nil
}

// RegisterDiscoveredAppSubRepos registers every sub-repo an app directory holds that workspace config does not already carry, reporting each outcome and skipping rather than failing on a single error.
func RegisterDiscoveredAppSubRepos(fs afero.Fs, out io.Writer, appName string) error {
	discovered, err := DiscoverAppSubRepos(fs, appName)
	if err != nil {
		return err
	}
	if len(discovered) == 0 {
		return nil
	}

	for _, sub := range discovered {

		config, err := ReadWorkspaceConfig(fs)
		if err != nil {
			return err
		}
		appIdx := FindAppIndex(config, appName)
		if appIdx == -1 {
			return fmt.Errorf("app not registered in workspace: %s", appName)
		}

		switch sub.Kind {
		case AppSubRepoKindService:
			if FindServiceIndex(config.Apps[appIdx], sub.Name) != -1 {
				fmt.Fprintf(out, "discovered service %s/%s: already registered, skipping\n", appName, sub.Name)
				continue
			}
			port, err := NextServicePort(fs, appName)
			if err != nil {
				fmt.Fprintf(out, "discovered service %s/%s: failed to allocate port: %v\n", appName, sub.Name, err)
				continue
			}
			image := DefaultServiceImage(appName, sub.Name)
			if err := AddServiceToWorkspace(fs, appName, sub.Name, port, image, "", ""); err != nil {
				fmt.Fprintf(out, "discovered service %s/%s: failed to register: %v\n", appName, sub.Name, err)
				continue
			}
			fmt.Fprintf(out, "discovered service %s/%s: registered\n", appName, sub.Name)

		case AppSubRepoKindLibrary:
			if FindAppLibraryIndex(config.Apps[appIdx], sub.Name) != -1 {
				fmt.Fprintf(out, "discovered library %s/%s: already registered, skipping\n", appName, sub.Name)
				continue
			}
			if err := AddAppLibraryToWorkspace(fs, appName, sub.Name); err != nil {
				fmt.Fprintf(out, "discovered library %s/%s: failed to register: %v\n", appName, sub.Name, err)
				continue
			}
			fmt.Fprintf(out, "discovered library %s/%s: registered\n", appName, sub.Name)

		case AppSubRepoKindProject:
			if FindAppProjectIndex(config.Apps[appIdx], sub.Name) != -1 {
				fmt.Fprintf(out, "discovered project %s/%s: already registered, skipping\n", appName, sub.Name)
				continue
			}
			if err := AddAppProjectToWorkspace(fs, appName, sub.Name); err != nil {
				fmt.Fprintf(out, "discovered project %s/%s: failed to register: %v\n", appName, sub.Name, err)
				continue
			}
			fmt.Fprintf(out, "discovered project %s/%s: registered\n", appName, sub.Name)

		case AppSubRepoKindTest:
			if FindAppTestIndex(config.Apps[appIdx], sub.Name) != -1 {
				fmt.Fprintf(out, "discovered test %s/%s: already registered, skipping\n", appName, sub.Name)
				continue
			}
			if err := AddTestToWorkspace(fs, appName, sub.Name); err != nil {
				fmt.Fprintf(out, "discovered test %s/%s: failed to register: %v\n", appName, sub.Name, err)
				continue
			}
			fmt.Fprintf(out, "discovered test %s/%s: registered\n", appName, sub.Name)
		}
	}
	return nil
}
