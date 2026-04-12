package functions

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/manifoldco/promptui"
)

func PromptForApp() (string, error) {
	apps, err := GetApps()
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
	services, err := GetAppServices(appName)
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
	libs, err := GetAppLibs(appName)
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

func PromptForTest(appName string) (string, error) {
	tests, err := GetAppTests(appName)
	if err != nil {
		return "", err
	}
	if len(tests) == 0 {
		return "", fmt.Errorf("no tests found for app: %s", appName)
	}
	items := make([]string, 0, len(tests))
	for _, entry := range tests {
		items = append(items, entry.Name)
	}
	return SelectFromList("Select test", items)
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
	libs, err := GetLibs()
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
	projects, err := GetProjects()
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

func PromptForTarget(config WorkspaceConfig, root string) (Target, error) {
	options := []string{}
	if len(config.Libraries) > 0 {
		options = append(options, "global library")
	}
	if len(config.Projects) > 0 {
		options = append(options, "project")
	}
	appLibApps := AppNamesWithLibraries(config)
	if len(appLibApps) > 0 {
		options = append(options, "app library")
	}
	serviceApps := AppNamesWithServices(config)
	if len(serviceApps) > 0 {
		options = append(options, "service")
	}
	testApps := AppNamesWithTests(config)
	if len(testApps) > 0 {
		options = append(options, "test")
	}
	templateNames := TemplateNames(config)
	if len(templateNames) > 0 {
		options = append(options, "template")
	}
	if len(options) == 0 {
		return Target{}, fmt.Errorf("no targets available")
	}
	targetKindLabel := options[0]
	if len(options) > 1 {
		selected, err := SelectFromList("Select target type", options)
		if err != nil {
			return Target{}, err
		}
		targetKindLabel = selected
	}
	switch targetKindLabel {
	case "service":
		appName, err := SelectFromList("Select app", serviceApps)
		if err != nil {
			return Target{}, err
		}
		serviceNames := ServiceNames(config, appName)
		if len(serviceNames) == 0 {
			return Target{}, fmt.Errorf("no services found for app: %s", appName)
		}
		name, err := SelectFromList("Select service", serviceNames)
		if err != nil {
			return Target{}, err
		}
		return Target{
			Kind: TargetService,
			App:  appName,
			Name: name,
			Path: filepath.Join(root, "repos", "apps", appName, "services", name),
		}, nil
	case "app library":
		appName, err := SelectFromList("Select app", appLibApps)
		if err != nil {
			return Target{}, err
		}
		libNames := AppLibraryNames(config, appName)
		if len(libNames) == 0 {
			return Target{}, fmt.Errorf("no libraries found for app: %s", appName)
		}
		name, err := SelectFromList("Select library", libNames)
		if err != nil {
			return Target{}, err
		}
		return Target{
			Kind: TargetAppLib,
			App:  appName,
			Name: name,
			Path: filepath.Join(root, "repos", "apps", appName, "libs", name),
		}, nil
	case "project":
		projectNames := ProjectNames(config)
		name, err := SelectFromList("Select project", projectNames)
		if err != nil {
			return Target{}, err
		}
		return Target{
			Kind: TargetProject,
			Name: name,
			Path: filepath.Join(root, "repos", "projects", name),
		}, nil
	case "test":
		appName, err := SelectFromList("Select app", testApps)
		if err != nil {
			return Target{}, err
		}
		testNames := TestNames(config, appName)
		if len(testNames) == 0 {
			return Target{}, fmt.Errorf("no tests found for app: %s", appName)
		}
		name, err := SelectFromList("Select test", testNames)
		if err != nil {
			return Target{}, err
		}
		return Target{
			Kind: TargetTest,
			App:  appName,
			Name: name,
			Path: filepath.Join(root, "repos", "apps", appName, "testing", name),
		}, nil
	case "global library":
		globalLibs := GlobalLibraryNames(config)
		name, err := SelectFromList("Select library", globalLibs)
		if err != nil {
			return Target{}, err
		}
		return Target{
			Kind: TargetGlobalLib,
			Name: name,
			Path: filepath.Join(root, "repos", "libs", name),
		}, nil
	case "template":
		name, err := SelectFromList("Select template", templateNames)
		if err != nil {
			return Target{}, err
		}
		return Target{
			Kind: TargetTemplate,
			Name: name,
			Path: filepath.Join(root, "repos", "templates", name),
		}, nil
	default:
		return Target{}, fmt.Errorf("unsupported target type")
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
