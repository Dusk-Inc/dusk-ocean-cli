package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/libraries"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/projects"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/scaffold"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/tree"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/workspace"
	"github.com/manifoldco/promptui"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var addAppCmd = &cobra.Command{
	Use:   "app",
	Short: "Add an app to repos/apps",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, err := cmd.Flags().GetString("name")
		if err != nil {
			return err
		}

		return scaffold.AddApp(afero.NewOsFs(), name)
	},
}

var addServiceCmd = &cobra.Command{
	Use:   "service",
	Short: "Add a service to an app",
	RunE: func(cmd *cobra.Command, args []string) error {
		apps, err := tree.GetApps()
		if err != nil {
			return err
		}
		if len(apps) == 0 {
			return fmt.Errorf("no apps found")
		}

		appItems := make([]string, 0, len(apps))
		for _, app := range apps {
			appItems = append(appItems, app.Name)
		}
		appPrompt := promptui.Select{
			Label: "Select app",
			Items: appItems,
		}
		_, appName, err := appPrompt.Run()
		if err != nil {
			return err
		}

		namePrompt := promptui.Prompt{
			Label: "Service name",
			Validate: func(input string) error {
				value := strings.TrimSpace(input)
				if value == "" {
					return fmt.Errorf("service name is required")
				}
				if strings.ContainsAny(value, " \t\n") {
					return fmt.Errorf("service name cannot include spaces")
				}
				for _, ch := range value {
					if (ch < 'a' || ch > 'z') && (ch < 'A' || ch > 'Z') {
						return fmt.Errorf("service name must only use letters")
					}
				}
				return nil
			},
		}
		serviceName, err := namePrompt.Run()
		if err != nil {
			return err
		}

		templates, err := tree.ListTemplatesByType("service")
		if err != nil {
			return err
		}
		if len(templates) == 0 {
			return fmt.Errorf("no api templates found")
		}
		templateItems := append([]string{"none (boilerplate)"}, templates...)
		templatePrompt := promptui.Select{
			Label: "Select template",
			Items: templateItems,
		}
		_, templateChoice, err := templatePrompt.Run()
		if err != nil {
			return err
		}
		template := ""
		if templateChoice != "none (boilerplate)" {
			template = templateChoice
		}

		root, err := tree.GetRoot()
		if err != nil {
			return err
		}
		containersRoot := filepath.Join(root, "repos", "containers")
		containerEntries, err := os.ReadDir(containersRoot)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("no Dockerfiles found")
			}
			return err
		}
		dockerfiles := make([]string, 0, len(containerEntries))
		for _, entry := range containerEntries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if strings.HasSuffix(name, "Dockerfile") {
				dockerfiles = append(dockerfiles, name)
			}
		}
		if len(dockerfiles) == 0 {
			return fmt.Errorf("no Dockerfiles found")
		}
		sort.Strings(dockerfiles)
		dockerPrompt := promptui.Select{
			Label: "Select Dockerfile",
			Items: dockerfiles,
		}
		_, dockerfileName, err := dockerPrompt.Run()
		if err != nil {
			return err
		}

		dbPrompt := promptui.Select{
			Label: "Attach database",
			Items: []string{"no", "yes"},
		}
		_, dbChoice, err := dbPrompt.Run()
		if err != nil {
			return err
		}
		attachDB := dbChoice == "yes"
		_ = attachDB

		fs := afero.NewOsFs()
		replacements := map[string]string{}
		if template != "" {
			templatePath := filepath.Join("repos", "templates", template)
			placeholders, err := collectPlaceholders(fs, templatePath)
			if err != nil {
				return err
			}
			replacements, err = promptPlaceholderValues(placeholders)
			if err != nil {
				return err
			}
		}
		return scaffold.AddService(fs, appName, serviceName, template, dockerfileName, replacements)
	},
}

var addLibCmd = &cobra.Command{
	Use:   "library",
	Short: "Add a library to an app or global registry",
	RunE: func(cmd *cobra.Command, args []string) error {
		locationPrompt := promptui.Select{
			Label: "Add library to",
			Items: []string{"global", "app"},
		}
		_, location, err := locationPrompt.Run()
		if err != nil {
			return err
		}

		appName := ""
		if location == "app" {
			apps, err := tree.GetApps()
			if err != nil {
				return err
			}
			if len(apps) == 0 {
				return fmt.Errorf("no apps found")
			}
			appItems := make([]string, 0, len(apps))
			for _, app := range apps {
				appItems = append(appItems, app.Name)
			}
			appPrompt := promptui.Select{
				Label: "Select app",
				Items: appItems,
			}
			_, appName, err = appPrompt.Run()
			if err != nil {
				return err
			}
		}

		root, err := tree.GetRoot()
		if err != nil {
			return err
		}
		templateItems, err := tree.ListTemplatesByType("library")
		if err != nil {
			return err
		}
		if len(templateItems) == 0 {
			return fmt.Errorf("no library templates found")
		}
		templatePrompt := promptui.Select{
			Label: "Select template",
			Items: templateItems,
		}
		_, templateName, err := templatePrompt.Run()
		if err != nil {
			return err
		}

		namePrompt := promptui.Prompt{
			Label: "Library name",
			Validate: func(input string) error {
				value := strings.TrimSpace(input)
				if value == "" {
					return fmt.Errorf("library name is required")
				}
				if strings.ContainsAny(value, " \t\n") {
					return fmt.Errorf("library name cannot include spaces")
				}
				for _, ch := range value {
					if (ch < 'a' || ch > 'z') && (ch < 'A' || ch > 'Z') && (ch < '0' || ch > '9') && ch != '-' && ch != '_' {
						return fmt.Errorf("library name must only use letters, numbers, dashes, and underscores")
					}
				}
				return nil
			},
		}
		libName, err := namePrompt.Run()
		if err != nil {
			return err
		}
		libName = strings.TrimSpace(libName)

		fs := afero.NewOsFs()
		destPath := ""
		if location == "app" {
			destPath = filepath.Join("repos", "apps", appName, "libs", libName)
		} else {
			destPath = filepath.Join("repos", "libs", libName)
		}
		if _, err := fs.Stat(destPath); err == nil {
			return fmt.Errorf("library already exists: %s", libName)
		} else if !os.IsNotExist(err) {
			return err
		}

		templatePath := filepath.Join("repos", "templates", templateName)
		if _, err := fs.Stat(templatePath); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("missing template: %s", templateName)
			}
			return err
		}
		placeholders, err := collectPlaceholders(fs, templatePath)
		if err != nil {
			return err
		}
		replacements, err := promptPlaceholderValues(placeholders)
		if err != nil {
			return err
		}
		if err := scaffold.CopyDirWithReplacements(fs, templatePath, destPath, replacements); err != nil {
			return err
		}

		templateConfig, err := workspace.ReadRepoConfig(fs, templatePath)
		if err != nil {
			return err
		}
		if templateConfig.Language == "go" {
			goModPath := filepath.Join(root, destPath, "go.mod")
			if info, err := os.Stat(goModPath); err == nil && !info.IsDir() {
				goCmd := exec.Command("go", "work", "use", filepath.Join(root, destPath))
				goCmd.Stdout = cmd.OutOrStdout()
				goCmd.Stderr = cmd.ErrOrStderr()
				goCmd.Dir = root
				if err := goCmd.Run(); err != nil {
					return err
				}
			}
		}

		if location == "app" {
			return libraries.AddAppLibraryToWorkspace(fs, appName, libName)
		}
		return libraries.AddGlobalLibraryToWorkspace(fs, libName)
	},
}

var addPkgCmd = &cobra.Command{
	Use:   "project",
	Short: "Add a project to repos/projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		templateItems, err := tree.ListTemplatesByType("project")
		if err != nil {
			return err
		}
		if len(templateItems) == 0 {
			return fmt.Errorf("no project templates found")
		}

		namePrompt := promptui.Prompt{
			Label: "Project name",
			Validate: func(input string) error {
				value := strings.TrimSpace(input)
				if value == "" {
					return fmt.Errorf("project name is required")
				}
				if strings.ContainsAny(value, " \t\n") {
					return fmt.Errorf("project name cannot include spaces")
				}
				for _, ch := range value {
					if (ch < 'a' || ch > 'z') && (ch < 'A' || ch > 'Z') && ch != '-' && ch != '_' {
						return fmt.Errorf("project name must only use letters, dashes, and underscores")
					}
				}
				return nil
			},
		}
		projectName, err := namePrompt.Run()
		if err != nil {
			return err
		}
		projectName = strings.TrimSpace(projectName)

		fs := afero.NewOsFs()
		destPath := filepath.Join("repos", "projects", projectName)
		if _, err := fs.Stat(destPath); err == nil {
			return fmt.Errorf("project already exists: %s", projectName)
		} else if !os.IsNotExist(err) {
			return err
		}

		templatePrompt := promptui.Select{
			Label: "Select template",
			Items: templateItems,
		}
		_, templateName, err := templatePrompt.Run()
		if err != nil {
			return err
		}

		templatePath := filepath.Join("repos", "templates", templateName)
		if _, err := fs.Stat(templatePath); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("missing template: %s", templateName)
			}
			return err
		}
		if err := scaffold.CopyDir(fs, templatePath, destPath); err != nil {
			return err
		}
		return projects.AddProjectToWorkspace(fs, projectName)
	},
}

var addTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Add a testing project to an app",
	RunE: func(cmd *cobra.Command, args []string) error {
		apps, err := tree.GetApps()
		if err != nil {
			return err
		}
		if len(apps) == 0 {
			return fmt.Errorf("no apps found")
		}

		appItems := make([]string, 0, len(apps))
		for _, app := range apps {
			appItems = append(appItems, app.Name)
		}
		appPrompt := promptui.Select{
			Label: "Select app",
			Items: appItems,
		}
		_, appName, err := appPrompt.Run()
		if err != nil {
			return err
		}

		namePrompt := promptui.Prompt{
			Label: "Test name",
			Validate: func(input string) error {
				value := strings.TrimSpace(input)
				if value == "" {
					return fmt.Errorf("test name is required")
				}
				if strings.ContainsAny(value, " \t\n") {
					return fmt.Errorf("test name cannot include spaces")
				}
				for _, ch := range value {
					if (ch < 'a' || ch > 'z') && (ch < 'A' || ch > 'Z') && (ch < '0' || ch > '9') && ch != '-' && ch != '_' {
						return fmt.Errorf("test name must only use letters, numbers, dashes, and underscores")
					}
				}
				return nil
			},
		}
		testName, err := namePrompt.Run()
		if err != nil {
			return err
		}
		testName = strings.TrimSpace(testName)

		templates, err := tree.ListTemplatesByType("test")
		if err != nil {
			return err
		}
		if len(templates) == 0 {
			return fmt.Errorf("no test templates found")
		}

		templatePrompt := promptui.Select{
			Label: "Select template",
			Items: templates,
		}
		_, templateName, err := templatePrompt.Run()
		if err != nil {
			return err
		}

		fs := afero.NewOsFs()
		destPath := filepath.Join("repos", "apps", appName, "testing", testName)
		if _, err := fs.Stat(destPath); err == nil {
			return fmt.Errorf("test already exists: %s", testName)
		} else if !os.IsNotExist(err) {
			return err
		}

		templatePath := filepath.Join("repos", "templates", templateName)
		if _, err := fs.Stat(templatePath); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("missing template: %s", templateName)
			}
			return err
		}

		placeholders, err := collectPlaceholders(fs, templatePath)
		if err != nil {
			return err
		}
		replacements := map[string]string{
			"test_name": testName,
			"app_name":  appName,
		}
		missing := make([]string, 0, len(placeholders))
		for _, placeholder := range placeholders {
			if _, ok := replacements[placeholder]; ok {
				continue
			}
			missing = append(missing, placeholder)
		}
		prompted, err := promptPlaceholderValues(missing)
		if err != nil {
			return err
		}
		for key, value := range prompted {
			replacements[key] = value
		}
		if err := scaffold.CopyDirWithReplacements(fs, templatePath, destPath, replacements); err != nil {
			return err
		}
		return workspace.AddTestToWorkspace(fs, appName, testName)
	},
}

func init() {
	addAppCmd.Flags().String("name", "", "Name of the app")
}

var placeholderPattern = regexp.MustCompile(`{{\s*([^{}]+?)\s*}}`)

func collectPlaceholders(fs afero.Fs, root string) ([]string, error) {
	placeholders := map[string]struct{}{}
	err := afero.Walk(fs, root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relPath != "." {
			addPlaceholders(relPath, placeholders)
		}
		if info.IsDir() {
			return nil
		}
		content, err := afero.ReadFile(fs, path)
		if err != nil {
			return err
		}
		addPlaceholders(string(content), placeholders)
		return nil
	})
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(placeholders))
	for key := range placeholders {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys, nil
}

func addPlaceholders(value string, placeholders map[string]struct{}) {
	matches := placeholderPattern.FindAllStringSubmatch(value, -1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		key := strings.TrimSpace(match[1])
		if key != "" {
			placeholders[key] = struct{}{}
		}
	}
}

func promptPlaceholderValues(placeholders []string) (map[string]string, error) {
	values := map[string]string{}
	for _, placeholder := range placeholders {
		prompt := promptui.Prompt{
			Label: fmt.Sprintf("Value for {{%s}}", placeholder),
			Validate: func(input string) error {
				if strings.TrimSpace(input) == "" {
					return fmt.Errorf("value is required")
				}
				return nil
			},
		}
		value, err := prompt.Run()
		if err != nil {
			return nil, err
		}
		values[placeholder] = value
	}
	return values, nil
}
