package cmd

import
(
	functions "github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/functions"
	tokens "github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

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

		return functions.AddApp(afero.NewOsFs(), name)
	},
}

var addServiceCmd = &cobra.Command{
	Use:   "service",
	Short: "Add a service to an app",
	RunE: func(cmd *cobra.Command, args []string) error {
		apps, err := functions.GetApps()
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

		templates, err := functions.ListTemplatesByType("service")
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

		root, err := functions.GetRoot()
		if err != nil {
			return err
		}
		containerFile, err := promptForContainerFile(root)
		if err != nil {
			return err
		}

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

		// REQ 19: pre-validate template-declared deps BEFORE any files
		// are copied, so an illegal propagation aborts cleanly with no
		// half-scaffolded state on disk.
		if template != "" {
			workspaceConfig, err := functions.ReadWorkspaceConfig(fs)
			if err != nil {
				return err
			}
			hypothetical := functions.Target{
				Kind: functions.TargetService,
				App:  appName,
				Name: serviceName,
				Path: filepath.Join(root, "repos", "apps", appName, "services", serviceName),
			}
			if err := functions.ValidateTemplateDepsForTarget(root, workspaceConfig, template, hypothetical); err != nil {
				return err
			}
		}

		if err := functions.AddService(fs, appName, serviceName, template, "", containerFile, replacements); err != nil {
			return err
		}
		servicePath := filepath.Join("repos", "apps", appName, "services", serviceName)
		if err := functions.RunSetupTask(fs, servicePath, root, cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
			return err
		}

		// REQ 19: propagate template deps after the new repo is registered.
		if template != "" {
			target := functions.Target{
				Kind: functions.TargetService,
				App:  appName,
				Name: serviceName,
				Path: filepath.Join(root, "repos", "apps", appName, "services", serviceName),
			}
			if err := functions.PropagateTemplateDeps(cmd, fs, template, target); err != nil {
				return err
			}
		}
		return nil
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
			apps, err := functions.GetApps()
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

		root, err := functions.GetRoot()
		if err != nil {
			return err
		}
		templateItems, err := functions.ListTemplatesByType("library")
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

		// REQ 19: pre-validate template-declared deps before copy.
		workspaceConfig, err := functions.ReadWorkspaceConfig(fs)
		if err != nil {
			return err
		}
		var hypothetical functions.Target
		if location == "app" {
			hypothetical = functions.Target{
				Kind: functions.TargetAppLib,
				App:  appName,
				Name: libName,
				Path: filepath.Join(root, "repos", "apps", appName, "libs", libName),
			}
		} else {
			hypothetical = functions.Target{
				Kind: functions.TargetGlobalLib,
				Name: libName,
				Path: filepath.Join(root, "repos", "libs", libName),
			}
		}
		if err := functions.ValidateTemplateDepsForTarget(root, workspaceConfig, templateName, hypothetical); err != nil {
			return err
		}

		if err := functions.CopyTemplate(fs, templatePath, destPath, replacements); err != nil {
			return err
		}

		if err := functions.RunSetupTask(fs, destPath, root, cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
			return err
		}

		if location == "app" {
			if err := functions.AddAppLibraryToWorkspace(fs, appName, libName); err != nil {
				return err
			}
		} else {
			if err := functions.AddGlobalLibraryToWorkspace(fs, libName); err != nil {
				return err
			}
		}

		// REQ 19: propagate template deps after registration.
		return functions.PropagateTemplateDeps(cmd, fs, templateName, hypothetical)
	},
}

var addPkgCmd = &cobra.Command{
	Use:   "project",
	Short: "Add a project to repos/projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		// REQ 19.6.1: project scaffolds may seed from service, library, or
		// project templates — apps remain non-template-able.
		templateItems, err := functions.ListTemplatesByType(
			tokens.TemplateKindService,
			tokens.TemplateKindLibrary,
			tokens.TemplateKindProject,
		)
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

		// REQ 19: pre-validate template-declared deps before copy.
		root, err := functions.GetRoot()
		if err != nil {
			return err
		}
		workspaceConfig, err := functions.ReadWorkspaceConfig(fs)
		if err != nil {
			return err
		}
		hypothetical := functions.Target{
			Kind: functions.TargetProject,
			Name: projectName,
			Path: filepath.Join(root, "repos", "projects", projectName),
		}
		if err := functions.ValidateTemplateDepsForTarget(root, workspaceConfig, templateName, hypothetical); err != nil {
			return err
		}

		if err := functions.CopyTemplate(fs, templatePath, destPath, nil); err != nil {
			return err
		}
		if err := functions.AddProjectToWorkspace(fs, projectName); err != nil {
			return err
		}
		return functions.PropagateTemplateDeps(cmd, fs, templateName, hypothetical)
	},
}

var addTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Add a testing project to an app",
	RunE: func(cmd *cobra.Command, args []string) error {
		apps, err := functions.GetApps()
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

		templates, err := functions.ListTemplatesByType("test")
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

		// REQ 19: pre-validate template-declared deps before copy.
		root, err := functions.GetRoot()
		if err != nil {
			return err
		}
		workspaceConfig, err := functions.ReadWorkspaceConfig(fs)
		if err != nil {
			return err
		}
		hypothetical := functions.Target{
			Kind: functions.TargetTest,
			App:  appName,
			Name: testName,
			Path: filepath.Join(root, "repos", "apps", appName, "testing", testName),
		}
		if err := functions.ValidateTemplateDepsForTarget(root, workspaceConfig, templateName, hypothetical); err != nil {
			return err
		}

		if err := functions.CopyTemplate(fs, templatePath, destPath, replacements); err != nil {
			return err
		}
		if err := functions.AddTestToWorkspace(fs, appName, testName); err != nil {
			return err
		}
		return functions.PropagateTemplateDeps(cmd, fs, templateName, hypothetical)
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

// reservedPlaceholderPrefix marks tokens that Dusk Ocean substitutes at
// runtime. These are NOT prompted at scaffold time and survive verbatim in
// the generated files so the runtime engine can fill them in later.
const reservedPlaceholderPrefix = tokens.VarNsOcean + ":"

func addPlaceholders(value string, placeholders map[string]struct{}) {
	matches := placeholderPattern.FindAllStringSubmatch(value, -1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		key := strings.TrimSpace(match[1])
		if key == "" {
			continue
		}
		// REQ 3.9: skip reserved {{ocean:*}} tokens — they are filled in
		// at runtime by Dusk Ocean, not at scaffold time by the user.
		if strings.HasPrefix(key, reservedPlaceholderPrefix) {
			continue
		}
		placeholders[key] = struct{}{}
	}
}

// promptForContainerFile asks the user how to assign a service's
// container_file (REQ 3.8). Three branches:
//
//   - existing: pick a file already present under repos/containers/.
//   - custom:   accept an arbitrary workspace-relative or bare path.
//   - none:     leave the field empty.
//
// The returned string goes straight into WorkspaceService.ContainerFile.
func promptForContainerFile(root string) (string, error) {
	containerOptionExisting := "existing"
	containerOptionCustom := "custom"
	containerOptionNone := "none"

	choicePrompt := promptui.Select{
		Label: "Container file",
		Items: []string{containerOptionExisting, containerOptionCustom, containerOptionNone},
	}
	_, choice, err := choicePrompt.Run()
	if err != nil {
		return "", err
	}

	switch choice {
	case containerOptionNone:
		return "", nil

	case containerOptionCustom:
		customPrompt := promptui.Prompt{
			Label: "Container file path (workspace-relative)",
			Validate: func(input string) error {
				if strings.TrimSpace(input) == "" {
					return fmt.Errorf("container file path is required")
				}
				return nil
			},
		}
		value, err := customPrompt.Run()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(value), nil

	case containerOptionExisting:
		containersRoot := filepath.Join(root, "repos", "containers")
		entries, err := os.ReadDir(containersRoot)
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		var files []string
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			files = append(files, entry.Name())
		}
		if len(files) == 0 {
			return "", fmt.Errorf("no files in repos/containers/; pick custom or none instead")
		}
		sort.Strings(files)
		filePrompt := promptui.Select{
			Label: "Select container file",
			Items: files,
		}
		_, file, err := filePrompt.Run()
		if err != nil {
			return "", err
		}
		return file, nil
	}

	return "", nil
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
