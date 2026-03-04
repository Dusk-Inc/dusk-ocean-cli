package cmd

import (
	functions "github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/functions"
	tokens "github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

type menuEntry struct {
	op          string
	description string
}

var menuEntries = []menuEntry{
	{tokens.MenuOpCreate, "Scaffold a new app or library"},
	{tokens.MenuOpRemove, "Delete an existing repo"},
	{tokens.MenuOpBuild, "Build a component"},
	{tokens.MenuOpCheck, "Run tests for a component"},
	{tokens.MenuOpRun, "Start a local development environment"},
	{tokens.MenuOpInstall, "Wire a local library dependency"},
	{tokens.MenuOpUninstall, "Unwire a local library dependency"},
	{tokens.MenuOpContain, "Build and push a Docker container image"},
	{tokens.MenuOpRefresh, "Install, build, and test the workspace"},
	{tokens.MenuOpVersion, "Show the CLI version"},
}

var menuCmd = &cobra.Command{
	Use:   "menu",
	Short: "Interactive guided menu for all CLI commands",
	Long:  `Provides a guided, interactive interface for all CLI commands. Scaffolding (menu create) and repo deletion (menu remove) are exclusively available through the menu.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		labels := make([]string, 0, len(menuEntries))
		for _, entry := range menuEntries {
			labels = append(labels, fmt.Sprintf("%-10s  %s", entry.op, entry.description))
		}

		prompt := promptui.Select{
			Label: "Select operation",
			Items: labels,
		}
		idx, _, err := prompt.Run()
		if err != nil {
			return err
		}

		selected := menuEntries[idx]
		fmt.Printf("\n%s: %s\n\n", selected.op, selected.description)

		switch selected.op {
		case tokens.MenuOpCreate:
			return menuCreateCmd.RunE(cmd, args)
		case tokens.MenuOpRemove:
			return menuRemoveCmd.RunE(cmd, args)
		case tokens.MenuOpBuild:
			return runMenuBuild(cmd)
		case tokens.MenuOpCheck:
			return runMenuCheck(cmd)
		case tokens.MenuOpRun:
			return runMenuRun(cmd)
		case tokens.MenuOpInstall:
			return runMenuInstall(cmd)
		case tokens.MenuOpUninstall:
			return runMenuUninstall(cmd)
		case tokens.MenuOpContain:
			return runMenuContain(cmd)
		case tokens.MenuOpRefresh:
			return runMenuRefresh(cmd)
		case tokens.MenuOpVersion:
			fmt.Println(functions.Version)
			return nil
		}
		return nil
	},
}

var menuCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Scaffold a new app or library (interactive, menu-only)",
	RunE: func(cmd *cobra.Command, args []string) error {
		typePrompt := promptui.Select{
			Label: "Create type",
			Items: []string{tokens.MenuTypeApp, tokens.MenuTypeLibrary},
		}
		_, createType, err := typePrompt.Run()
		if err != nil {
			return err
		}

		fs := afero.NewOsFs()

		switch createType {
		case tokens.MenuTypeApp:
			return runMenuCreateApp(fs)
		case tokens.MenuTypeLibrary:
			return runMenuCreateLibrary(cmd, fs)
		}
		return nil
	},
}

var menuRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Delete an existing repo (interactive, menu-only)",
	RunE: func(cmd *cobra.Command, args []string) error {
		typePrompt := promptui.Select{
			Label: "Remove type",
			Items: []string{
				tokens.MenuTypeApp,
				tokens.MenuTypeService,
				tokens.MenuTypeLibrary,
				tokens.MenuTypeProject,
				tokens.MenuTypeTest,
			},
		}
		_, removeType, err := typePrompt.Run()
		if err != nil {
			return err
		}

		fs := afero.NewOsFs()

		switch removeType {
		case tokens.MenuTypeApp:
			return runMenuRemoveApp(cmd, fs)
		case tokens.MenuTypeService:
			return runMenuRemoveService(cmd, fs)
		case tokens.MenuTypeLibrary:
			return runMenuRemoveLibrary(cmd, fs)
		case tokens.MenuTypeProject:
			return runMenuRemoveProject(cmd, fs)
		case tokens.MenuTypeTest:
			return runMenuRemoveTest(cmd, fs)
		}
		return nil
	},
}

// runMenuCreateApp scaffolds a new app (REQ 3.2).
func runMenuCreateApp(fs afero.Fs) error {
	namePrompt := promptui.Prompt{
		Label: "App name",
		Validate: func(input string) error {
			value := strings.TrimSpace(input)
			if value == "" {
				return fmt.Errorf("app name is required")
			}
			if strings.ContainsAny(value, " \t\n") {
				return fmt.Errorf("app name cannot include spaces")
			}
			for _, ch := range value {
				if (ch < 'a' || ch > 'z') && (ch < 'A' || ch > 'Z') && (ch < '0' || ch > '9') && ch != '-' && ch != '_' {
					return fmt.Errorf("app name must only use letters, numbers, dashes, and underscores")
				}
			}
			return nil
		},
	}
	name, err := namePrompt.Run()
	if err != nil {
		return err
	}
	return functions.AddApp(fs, strings.TrimSpace(name))
}

// runMenuCreateLibrary scaffolds a new global or app-adjacent library (REQ 3.3, 3.4).
func runMenuCreateLibrary(cmd *cobra.Command, fs afero.Fs) error {
	locationPrompt := promptui.Select{
		Label: "Add library to",
		Items: []string{tokens.MenuLibGlobal, tokens.MenuTypeApp},
	}
	_, location, err := locationPrompt.Run()
	if err != nil {
		return err
	}

	appName := ""
	if location == tokens.MenuTypeApp {
		selected, err := functions.PromptForApp()
		if err != nil {
			return err
		}
		appName = selected
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

	var destPath string
	if location == tokens.MenuTypeApp {
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
	if err := functions.CopyDirWithReplacements(fs, templatePath, destPath, replacements); err != nil {
		return err
	}

	templateConfig, err := functions.ReadRepoConfig(fs, templatePath)
	if err != nil {
		return err
	}
	if templateConfig.Language == "go" {
		root, err := functions.GetRoot()
		if err != nil {
			return err
		}
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

	if location == tokens.MenuTypeApp {
		return functions.AddAppLibraryToWorkspace(fs, appName, libName)
	}
	return functions.AddGlobalLibraryToWorkspace(fs, libName)
}

// runMenuRemoveApp removes an app (REQ 3.5).
func runMenuRemoveApp(cmd *cobra.Command, fs afero.Fs) error {
	name, err := functions.PromptForApp()
	if err != nil {
		return err
	}

	confirm, err := confirmRemoval(fmt.Sprintf("Remove app %q? This action is permanent", name))
	if err != nil {
		return err
	}
	if !confirm {
		return fmt.Errorf(tokens.ErrAborted)
	}

	return functions.RemoveApp(fs, name)
}

// runMenuRemoveService removes a service from an app (REQ 3.5).
func runMenuRemoveService(cmd *cobra.Command, fs afero.Fs) error {
	appName, err := functions.PromptForApp()
	if err != nil {
		return err
	}

	name, err := functions.PromptForService(appName)
	if err != nil {
		return err
	}

	confirm, err := confirmRemoval(fmt.Sprintf("Remove service %q from app %q? This action is permanent", name, appName))
	if err != nil {
		return err
	}
	if !confirm {
		return fmt.Errorf(tokens.ErrAborted)
	}

	root, err := functions.GetRoot()
	if err != nil {
		return err
	}
	servicePath := filepath.Join(root, "repos", "apps", appName, "services", name)
	if _, err := fs.Stat(servicePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("service does not exist: %s", name)
		}
		return err
	}
	if err := fs.RemoveAll(servicePath); err != nil {
		return err
	}
	return functions.RemoveServiceFromWorkspace(fs, appName, name)
}

// runMenuRemoveLibrary removes a global or app-adjacent library, handling dependents (REQ 3.5, 3.6).
func runMenuRemoveLibrary(cmd *cobra.Command, fs afero.Fs) error {
	locationPrompt := promptui.Select{
		Label: "Remove library from",
		Items: []string{tokens.MenuLibGlobal, tokens.MenuTypeApp},
	}
	_, location, err := locationPrompt.Run()
	if err != nil {
		return err
	}

	appName := ""
	var name string

	if location == tokens.MenuTypeApp {
		selected, err := functions.PromptForApp()
		if err != nil {
			return err
		}
		appName = selected

		selectedLib, err := functions.PromptForAppLib(appName)
		if err != nil {
			return err
		}
		name = selectedLib
	} else {
		selectedLib, err := functions.PromptForGlobalLib()
		if err != nil {
			return err
		}
		name = selectedLib
	}

	label := fmt.Sprintf("Remove library %q? This action is permanent", name)
	if location == tokens.MenuTypeApp {
		label = fmt.Sprintf("Remove library %q from app %q? This action is permanent", name, appName)
	}
	confirm, err := confirmRemoval(label)
	if err != nil {
		return err
	}
	if !confirm {
		return fmt.Errorf(tokens.ErrAborted)
	}

	root, err := functions.GetRoot()
	if err != nil {
		return err
	}

	var libPath string
	if location == tokens.MenuTypeApp {
		libPath = filepath.Join(root, "repos", "apps", appName, "libs", name)
	} else {
		libPath = filepath.Join(root, "repos", "libs", name)
	}

	if _, err := fs.Stat(libPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("library does not exist: %s", name)
		}
		return err
	}

	source := tokens.MenuLibGlobal
	if location == tokens.MenuTypeApp {
		source = appName
	}

	workspaceConfig, err := functions.ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}

	dependents := functions.CollectLibraryDependents(workspaceConfig, root, name, source)
	if len(dependents) > 0 {
		for _, target := range dependents {
			if _, err := fs.Stat(target.Path); err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("dependency target does not exist: %s", target.Path)
				}
				return err
			}
		}
		if err := functions.RunUninstallForTargets(cmd, fs, libPath, name, dependents, functions.UninstallOptions{}); err != nil {
			return err
		}
	}

	if err := fs.RemoveAll(libPath); err != nil {
		return err
	}

	return functions.UpdateConfig(fs, func(config functions.WorkspaceConfig) (functions.WorkspaceConfig, error) {
		config = functions.RemoveLibraryDeps(config, name, source)
		if location == tokens.MenuTypeApp {
			return functions.RemoveAppLibraryFromConfig(config, appName, name), nil
		}
		return functions.RemoveGlobalLibraryFromConfig(config, name), nil
	})
}

// runMenuRemoveProject removes a project (REQ 3.5).
func runMenuRemoveProject(cmd *cobra.Command, fs afero.Fs) error {
	name, err := functions.PromptForProject()
	if err != nil {
		return err
	}

	confirm, err := confirmRemoval(fmt.Sprintf("Remove project %q? This action is permanent", name))
	if err != nil {
		return err
	}
	if !confirm {
		return fmt.Errorf(tokens.ErrAborted)
	}

	root, err := functions.GetRoot()
	if err != nil {
		return err
	}
	projectPath := filepath.Join(root, "repos", "projects", name)
	if _, err := fs.Stat(projectPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("project does not exist: %s", name)
		}
		return err
	}
	if err := fs.RemoveAll(projectPath); err != nil {
		return err
	}
	return functions.RemoveProjectFromWorkspace(fs, name)
}

// runMenuRemoveTest removes a testing project from an app (REQ 3.5).
func runMenuRemoveTest(cmd *cobra.Command, fs afero.Fs) error {
	appName, err := functions.PromptForApp()
	if err != nil {
		return err
	}

	name, err := functions.PromptForTest(appName)
	if err != nil {
		return err
	}

	confirm, err := confirmRemoval(fmt.Sprintf("Remove test %q from app %q? This action is permanent", name, appName))
	if err != nil {
		return err
	}
	if !confirm {
		return fmt.Errorf(tokens.ErrAborted)
	}

	root, err := functions.GetRoot()
	if err != nil {
		return err
	}
	testPath := filepath.Join(root, "repos", "apps", appName, "testing", name)
	if _, err := fs.Stat(testPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("test does not exist: %s", name)
		}
		return err
	}
	if err := fs.RemoveAll(testPath); err != nil {
		return err
	}
	return functions.RemoveTestFromWorkspace(fs, appName, name)
}

// confirmRemoval shows a confirmation prompt and returns true if the user confirms with 'y' (REQ 3.7).
func confirmRemoval(label string) (bool, error) {
	prompt := promptui.Prompt{
		Label:     label,
		IsConfirm: true,
	}
	answer, err := prompt.Run()
	if err != nil {
		if err == promptui.ErrAbort {
			return false, nil
		}
		return false, err
	}
	return strings.ToLower(answer) == tokens.ConfirmYes, nil
}

// runMenuBuild shows a target selector and builds it with dependencies (REQ 3.1).
func runMenuBuild(cmd *cobra.Command) error {
	fs := afero.NewOsFs()
	config, err := functions.ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	root, err := functions.GetRoot()
	if err != nil {
		return err
	}

	target, err := functions.PromptForTarget(config, root)
	if err != nil {
		return err
	}

	built := map[string]struct{}{}
	switch target.Kind {
	case functions.TargetService:
		node, err := functions.MakeServiceNode(config, target.App, target.Name)
		if err != nil {
			return err
		}
		return functions.RunBuildWithDependencies(cmd, root, config, node, built)
	case functions.TargetAppLib:
		node, err := functions.MakeAppLibNode(config, target.App, target.Name)
		if err != nil {
			return err
		}
		return functions.RunBuildWithDependencies(cmd, root, config, node, built)
	case functions.TargetGlobalLib:
		node, err := functions.MakeGlobalLibNode(config, target.Name)
		if err != nil {
			return err
		}
		return functions.RunBuildWithDependencies(cmd, root, config, node, built)
	case functions.TargetProject:
		node, err := functions.MakeProjectNode(config, target.Name)
		if err != nil {
			return err
		}
		return functions.RunBuildWithDependencies(cmd, root, config, node, built)
	case functions.TargetTest:
		node, err := functions.MakeTestNode(config, target.App, target.Name)
		if err != nil {
			return err
		}
		return functions.RunBuildWithDependencies(cmd, root, config, node, built)
	}
	return fmt.Errorf("unsupported target type")
}

// runMenuCheck shows a target selector and runs tests with dependencies (REQ 3.1).
func runMenuCheck(cmd *cobra.Command) error {
	fs := afero.NewOsFs()
	config, err := functions.ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	root, err := functions.GetRoot()
	if err != nil {
		return err
	}

	target, err := functions.PromptForTarget(config, root)
	if err != nil {
		return err
	}

	built := map[string]struct{}{}
	var passThrough []string

	switch target.Kind {
	case functions.TargetService:
		node, err := functions.MakeServiceNode(config, target.App, target.Name)
		if err != nil {
			return err
		}
		return functions.RunCheckWithDependencies(cmd, root, config, node, built, passThrough)
	case functions.TargetAppLib:
		node, err := functions.MakeAppLibNode(config, target.App, target.Name)
		if err != nil {
			return err
		}
		return functions.RunCheckWithDependencies(cmd, root, config, node, built, passThrough)
	case functions.TargetGlobalLib:
		node, err := functions.MakeGlobalLibNode(config, target.Name)
		if err != nil {
			return err
		}
		return functions.RunCheckWithDependencies(cmd, root, config, node, built, passThrough)
	case functions.TargetProject:
		node, err := functions.MakeProjectNode(config, target.Name)
		if err != nil {
			return err
		}
		return functions.RunCheckWithDependencies(cmd, root, config, node, built, passThrough)
	case functions.TargetTest:
		node, err := functions.MakeTestNode(config, target.App, target.Name)
		if err != nil {
			return err
		}
		return functions.RunCheckWithDependencies(cmd, root, config, node, built, passThrough)
	}
	return fmt.Errorf("unsupported target type")
}

// runMenuRun shows a run-type selector and starts the selected app or services (REQ 3.1).
func runMenuRun(cmd *cobra.Command) error {
	runTypePrompt := promptui.Select{
		Label: "Run type",
		Items: []string{tokens.MenuTypeApp, tokens.MenuTypeService},
	}
	_, runType, err := runTypePrompt.Run()
	if err != nil {
		return err
	}

	appName, err := functions.PromptForApp()
	if err != nil {
		return err
	}

	root, err := functions.GetRoot()
	if err != nil {
		return err
	}
	appPath := filepath.Join(root, "repos", "apps", appName)

	composeArgs := []string{"compose", "-f", "docker-compose.yml", "-f", "docker-compose.dev.yml"}

	if runType == tokens.MenuTypeService {
		services, err := functions.GetAppServices(appName)
		if err != nil {
			return err
		}
		if len(services) == 0 {
			return fmt.Errorf("no services found for app: %s", appName)
		}

		items := make([]string, 0, len(services)+1)
		for _, service := range services {
			items = append(items, service.Name)
		}
		items = append(items, "confirm")

		selected := []string{}
		selectedSet := map[string]bool{}
		for {
			prompt := promptui.Select{
				Label: "Select services",
				Items: items,
			}
			_, name, err := prompt.Run()
			if err != nil {
				return err
			}
			if name == "confirm" {
				if len(selected) == 0 {
					return fmt.Errorf("select at least one service")
				}
				break
			}
			if selectedSet[name] {
				selectedSet[name] = false
				next := make([]string, 0, len(selected)-1)
				for _, entry := range selected {
					if entry != name {
						next = append(next, entry)
					}
				}
				selected = next
			} else {
				selectedSet[name] = true
				selected = append(selected, name)
			}
		}
		composeArgs = append(composeArgs, "up")
		composeArgs = append(composeArgs, selected...)
	} else {
		composeArgs = append(composeArgs, "up")
	}

	execCmd := exec.Command("docker", composeArgs...)
	execCmd.Dir = appPath
	execCmd.Stdout = cmd.OutOrStdout()
	execCmd.Stderr = cmd.ErrOrStderr()
	execCmd.Stdin = cmd.InOrStdin()
	return execCmd.Run()
}

// runMenuInstall delegates to the existing interactive install prompt (REQ 3.1).
func runMenuInstall(cmd *cobra.Command) error {
	return functions.RunInstallPrompt(cmd, afero.NewOsFs())
}

// runMenuUninstall delegates to the existing interactive uninstall prompt (REQ 3.1).
func runMenuUninstall(cmd *cobra.Command) error {
	return functions.RunUninstallPrompt(cmd, afero.NewOsFs())
}

// runMenuContain prompts for an app and service and builds + pushes the container image (REQ 3.1).
func runMenuContain(cmd *cobra.Command) error {
	appName, err := functions.PromptForApp()
	if err != nil {
		return err
	}

	serviceName, err := functions.PromptForService(appName)
	if err != nil {
		return err
	}

	root, err := functions.GetRoot()
	if err != nil {
		return err
	}
	servicePath := filepath.Join(root, "repos", "apps", appName, "services", serviceName)

	imageName, err := functions.ServiceImageReference(afero.NewOsFs(), appName, serviceName)
	if err != nil {
		return err
	}

	buildCmd := exec.Command("docker", "build", "-t", imageName, ".")
	buildCmd.Dir = servicePath
	buildCmd.Stdout = cmd.OutOrStdout()
	buildCmd.Stderr = cmd.ErrOrStderr()
	buildCmd.Stdin = cmd.InOrStdin()
	if err := buildCmd.Run(); err != nil {
		return err
	}

	pushCmd := exec.Command("docker", "push", imageName)
	pushCmd.Stdout = cmd.OutOrStdout()
	pushCmd.Stderr = cmd.ErrOrStderr()
	pushCmd.Stdin = cmd.InOrStdin()
	return pushCmd.Run()
}

// runMenuRefresh prompts for hash-clearing preference and runs a full workspace refresh (REQ 3.1).
func runMenuRefresh(cmd *cobra.Command) error {
	clearPrompt := promptui.Select{
		Label: "Clear build/check hashes before refresh",
		Items: []string{"no", "yes"},
	}
	_, clearChoice, err := clearPrompt.Run()
	if err != nil {
		return err
	}

	fs := afero.NewOsFs()
	root, err := functions.GetRoot()
	if err != nil {
		return err
	}

	if clearChoice == "yes" {
		if err := functions.ClearHashes(fs, cmd, root); err != nil {
			return err
		}
	}

	if err := functions.ValidateComposeConsistency(fs, root); err != nil {
		return err
	}

	config, err := functions.ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	if err := functions.RunRefresh(cmd, fs, root, config); err != nil {
		return err
	}
	return functions.CleanupStaleHashes(fs, cmd, root)
}
