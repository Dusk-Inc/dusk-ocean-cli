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
	prompt := promptui.Select{
		Label: "Select app",
		Items: items,
	}
	_, name, err := prompt.Run()
	if err != nil {
		return "", err
	}
	return name, nil
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
	prompt := promptui.Select{
		Label: "Select service",
		Items: items,
	}
	_, name, err := prompt.Run()
	if err != nil {
		return "", err
	}
	return name, nil
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
	prompt := promptui.Select{
		Label: "Select library",
		Items: items,
	}
	_, name, err := prompt.Run()
	if err != nil {
		return "", err
	}
	return name, nil
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
	prompt := promptui.Select{
		Label: "Select language",
		Items: languages,
	}
	_, language, err := prompt.Run()
	if err != nil {
		return "", err
	}
	return language, nil
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
	prompt := promptui.Select{
		Label: "Select library",
		Items: items,
	}
	_, name, err := prompt.Run()
	if err != nil {
		return "", err
	}
	return name, nil
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
	prompt := promptui.Select{
		Label: "Select project",
		Items: items,
	}
	_, name, err := prompt.Run()
	if err != nil {
		return "", err
	}
	return name, nil
}
