package prompts

import (
	"fmt"
	"os"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/tree"
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
