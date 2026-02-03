package prompts

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/tree"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/workspace"
	"github.com/manifoldco/promptui"
)

func PromptForApp() (string, error) {
	apps, err := tree.GetApps()
	if err != nil {
		return "", err
	}
	if len(apps) == 0 {
		return "", fmt.Errorf("no apps found")
	}
	items := make([]string, 0, len(apps))
	for _, app := range apps {
		items = append(items, app.Name)
	}
	return SelectFromList("Select app", items)
}

func PromptForService(appName string) (string, error) {
	services, err := tree.GetAppServices(appName)
	if err != nil {
		return "", err
	}
	if len(services) == 0 {
		return "", fmt.Errorf("no services found for app: %s", appName)
	}
	items := make([]string, 0, len(services))
	for _, service := range services {
		items = append(items, service.Name)
	}
	return SelectFromList("Select service", items)
}

func PromptForAppLib(appName string) (string, error) {
	libs, err := tree.GetAppLibs(appName)
	if err != nil {
		return "", err
	}
	if len(libs) == 0 {
		return "", fmt.Errorf("no libraries found for app: %s", appName)
	}
	items := make([]string, 0, len(libs))
	for _, lib := range libs {
		items = append(items, lib.Name)
	}
	return SelectFromList("Select library", items)
}

func PromptForLanguage(rootPath string) (string, error) {
	entries, err := os.ReadDir(rootPath)
	if err != nil {
		return "", err
	}
	var languages []string
	for _, entry := range entries {
		if entry.IsDir() {
			languages = append(languages, entry.Name())
		}
	}
	if len(languages) == 0 {
		return "", fmt.Errorf("no languages found in %s", rootPath)
	}
	return SelectFromList("Select language", languages)
}

func PromptForGlobalLib() (string, error) {
	libs, err := tree.GetLibs()
	if err != nil {
		return "", err
	}
	if len(libs) == 0 {
		return "", fmt.Errorf("no libraries found")
	}
	items := make([]string, 0, len(libs))
	for _, lib := range libs {
		items = append(items, lib.Name)
	}
	return SelectFromList("Select library", items)
}

func PromptForProject() (string, error) {
	projects, err := tree.GetProjects()
	if err != nil {
		return "", err
	}
	if len(projects) == 0 {
		return "", fmt.Errorf("no projects found")
	}
	items := make([]string, 0, len(projects))
	for _, project := range projects {
		items = append(items, project.Name)
	}
	return SelectFromList("Select project", items)
}

func PromptForTarget(config workspace.WorkspaceConfig, root string) (workspace.Target, error) {
	options := []string{}
	if len(config.Libraries) > 0 {
		options = append(options, "global library")
	}
	if len(config.Projects) > 0 {
		options = append(options, "project")
	}
	appLibApps := workspace.AppNamesWithLibraries(config)
	if len(appLibApps) > 0 {
		options = append(options, "app library")
	}
	serviceApps := workspace.AppNamesWithServices(config)
	if len(serviceApps) > 0 {
		options = append(options, "service")
	}
	if len(options) == 0 {
		return workspace.Target{}, fmt.Errorf("no targets available")
	}
	targetKindLabel := options[0]
	if len(options) > 1 {
		selected, err := SelectFromList("Select target type", options)
		if err != nil {
			return workspace.Target{}, err
		}
		targetKindLabel = selected
	}
	switch targetKindLabel {
	case "service":
		appName, err := SelectFromList("Select app", serviceApps)
		if err != nil {
			return workspace.Target{}, err
		}
		serviceNames := workspace.ServiceNames(config, appName)
		if len(serviceNames) == 0 {
			return workspace.Target{}, fmt.Errorf("no services found for app: %s", appName)
		}
		name, err := SelectFromList("Select service", serviceNames)
		if err != nil {
			return workspace.Target{}, err
		}
		return workspace.Target{
			Kind: workspace.TargetService,
			App:  appName,
			Name: name,
			Path: filepath.Join(root, "repos", "apps", appName, "services", name),
		}, nil
	case "app library":
		appName, err := SelectFromList("Select app", appLibApps)
		if err != nil {
			return workspace.Target{}, err
		}
		libNames := workspace.AppLibraryNames(config, appName)
		if len(libNames) == 0 {
			return workspace.Target{}, fmt.Errorf("no libraries found for app: %s", appName)
		}
		name, err := SelectFromList("Select library", libNames)
		if err != nil {
			return workspace.Target{}, err
		}
		return workspace.Target{
			Kind: workspace.TargetAppLib,
			App:  appName,
			Name: name,
			Path: filepath.Join(root, "repos", "apps", appName, "libs", name),
		}, nil
	case "project":
		projectNames := workspace.ProjectNames(config)
		name, err := SelectFromList("Select project", projectNames)
		if err != nil {
			return workspace.Target{}, err
		}
		return workspace.Target{
			Kind: workspace.TargetProject,
			Name: name,
			Path: filepath.Join(root, "repos", "projects", name),
		}, nil
	case "global library":
		globalLibs := workspace.GlobalLibraryNames(config)
		name, err := SelectFromList("Select library", globalLibs)
		if err != nil {
			return workspace.Target{}, err
		}
		return workspace.Target{
			Kind: workspace.TargetGlobalLib,
			Name: name,
			Path: filepath.Join(root, "repos", "libs", name),
		}, nil
	default:
		return workspace.Target{}, fmt.Errorf("unsupported target type")
	}
}

// SelectFromList displays a prompt selection for a list of items.
func SelectFromList(label string, items []string) (string, error) {
	if len(items) == 0 {
		return "", fmt.Errorf("no options available")
	}
	prompt := promptui.Select{
		Label: label,
		Items: items,
	}
	_, selected, err := prompt.Run()
	if err != nil {
		return "", err
	}
	return selected, nil
}
