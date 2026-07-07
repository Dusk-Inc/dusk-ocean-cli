package cmd

import (
	"fmt"
	functions "github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/functions"
	tokens "github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
	"os"
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
	{tokens.MenuOpCreate, "Scaffold a new component"},
	{tokens.MenuOpRemove, "Delete an existing repo"},
	{tokens.MenuOpBuild, "Build a component"},
	{tokens.MenuOpCheck, "Run tests for a component"},
	{tokens.MenuOpRun, "Start a local development environment"},
	{tokens.MenuOpInstall, "Wire a local library dependency"},
	{tokens.MenuOpUninstall, "Unwire a local library dependency"},
	{tokens.MenuOpContain, "Build and push a Docker container image"},
	{tokens.MenuOpRename, "Rename a repository"},
	{tokens.MenuOpRefresh, "Install, build, and test the workspace"},
	{tokens.MenuOpAdopt, "Clone an external git repo into the workspace"},
	{tokens.MenuOpRegister, "Register an already-on-disk repo into the workspace"},
	{tokens.MenuOpTask, "Run a workspace-level task against a repo"},
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
		case tokens.MenuOpRename:
			return runMenuRename(cmd)
		case tokens.MenuOpRefresh:
			return runMenuRefresh(cmd)
		case tokens.MenuOpAdopt:
			return runMenuAdopt(cmd)
		case tokens.MenuOpRegister:
			return runMenuRegister(cmd)
		case tokens.MenuOpTask:
			return runMenuTask(cmd)
		case tokens.MenuOpVersion:
			fmt.Println(functions.Version)
			return nil
		}
		return nil
	},
}

var menuCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Scaffold a new component (interactive, menu-only)",
	RunE: func(cmd *cobra.Command, args []string) error {
		typePrompt := promptui.Select{
			Label: "Create type",
			Items: []string{
				tokens.MenuTypeApp,
				tokens.MenuTypeService,
				tokens.MenuTypeLibrary,
				tokens.MenuTypeProject,
				tokens.MenuTypeInfra,
				tokens.MenuTypeDocs,
			},
		}
		_, createType, err := typePrompt.Run()
		if err != nil {
			return err
		}

		fs := afero.NewOsFs()

		switch createType {
		case tokens.MenuTypeApp:
			return runMenuCreateApp(fs, cmd)
		case tokens.MenuTypeService:
			return addServiceCmd.RunE(cmd, args)
		case tokens.MenuTypeLibrary:
			return addLibCmd.RunE(cmd, args)
		case tokens.MenuTypeProject:
			return addPkgCmd.RunE(cmd, args)
		case tokens.MenuTypeInfra:
			return addInfraCmd.RunE(cmd, args)
		case tokens.MenuTypeDocs:
			return addDocsCmd.RunE(cmd, args)
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
				tokens.MenuTypeInfra,
				tokens.MenuTypeDocs,
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
		case tokens.MenuTypeInfra:
			return removeInfraCmd.RunE(cmd, args)
		case tokens.MenuTypeDocs:
			return removeDocsCmd.RunE(cmd, args)
		}
		return nil
	},
}

func runMenuCreateApp(fs afero.Fs, cmd *cobra.Command) error {
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
	rawName, err := namePrompt.Run()
	if err != nil {
		return err
	}
	name := strings.TrimSpace(rawName)
	if err := functions.AddApp(fs, name); err != nil {
		return err
	}
	return functions.WireNewRepoVcs(fs, cmd.OutOrStdout(), cmd.ErrOrStderr(), tokens.RepoKindApp, name, "")
}

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

func runMenuRename(cmd *cobra.Command) error {
	typePrompt := promptui.Select{
		Label: "Rename type",
		Items: []string{tokens.MenuTypeApp, tokens.MenuTypeService, tokens.MenuTypeLibrary, tokens.MenuTypeProject},
	}
	_, renameType, err := typePrompt.Run()
	if err != nil {
		return err
	}

	var repoName string
	var inApp string

	switch renameType {
	case tokens.MenuTypeApp:
		repoName, err = functions.PromptForApp()
		if err != nil {
			return err
		}

	case tokens.MenuTypeService:
		inApp, err = functions.PromptForApp()
		if err != nil {
			return err
		}
		repoName, err = functions.PromptForService(inApp)
		if err != nil {
			return err
		}

	case tokens.MenuTypeLibrary:
		locationPrompt := promptui.Select{
			Label: "Library location",
			Items: []string{tokens.MenuLibGlobal, tokens.MenuTypeApp},
		}
		_, location, err := locationPrompt.Run()
		if err != nil {
			return err
		}
		if location == tokens.MenuTypeApp {
			inApp, err = functions.PromptForApp()
			if err != nil {
				return err
			}
			repoName, err = functions.PromptForAppLib(inApp)
			if err != nil {
				return err
			}
		} else {
			repoName, err = functions.PromptForGlobalLib()
			if err != nil {
				return err
			}
		}

	case tokens.MenuTypeProject:
		repoName, err = functions.PromptForProject()
		if err != nil {
			return err
		}
	}

	newNamePrompt := promptui.Prompt{
		Label: "New name",
		Validate: func(input string) error {
			value := strings.TrimSpace(input)
			if value == "" {
				return fmt.Errorf("new name is required")
			}
			if strings.ContainsAny(value, " \t\n") {
				return fmt.Errorf("name cannot include spaces")
			}
			for _, ch := range value {
				if (ch < 'a' || ch > 'z') && (ch < 'A' || ch > 'Z') && (ch < '0' || ch > '9') && ch != '-' && ch != '_' {
					return fmt.Errorf("name must only use letters, numbers, dashes, and underscores")
				}
			}
			return nil
		},
	}
	newName, err := newNamePrompt.Run()
	if err != nil {
		return err
	}

	if inApp != "" {
		return functions.RenameRepo(cmd, afero.NewOsFs(), repoName, strings.TrimSpace(newName), inApp)
	}
	return functions.RenameRepo(cmd, afero.NewOsFs(), repoName, strings.TrimSpace(newName))
}

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

func runMenuRun(cmd *cobra.Command) error {
	fs := afero.NewOsFs()

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

	if runType == tokens.MenuTypeService {
		serviceName, err := functions.PromptForService(appName)
		if err != nil {
			return err
		}
		return functions.RunService(cmd, fs, appName, serviceName, false, groupSelection(cmd))
	}

	return functions.RunApp(cmd, fs, appName, false, groupSelection(cmd))
}

func runMenuInstall(cmd *cobra.Command) error {
	return functions.RunInstallPrompt(cmd, afero.NewOsFs())
}

func runMenuUninstall(cmd *cobra.Command) error {
	return functions.RunUninstallPrompt(cmd, afero.NewOsFs())
}

func runMenuContain(cmd *cobra.Command) error {
	appName, err := functions.PromptForApp()
	if err != nil {
		return err
	}

	serviceName, err := functions.PromptForService(appName)
	if err != nil {
		return err
	}

	return functions.ContainService(cmd, afero.NewOsFs(), appName, serviceName)
}

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

	config, err := functions.ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	if err := functions.RunRefresh(cmd, fs, root, config); err != nil {
		return err
	}
	return functions.CleanupStaleHashes(fs, cmd, root)
}

func promptForRepoKind() (kind string, app string, templateKind string, err error) {
	kindPrompt := promptui.Select{
		Label: "Repo kind",
		Items: []string{
			tokens.RepoKindProject,
			tokens.RepoKindLibrary,
			tokens.RepoKindApp,
			tokens.RepoKindService,
			tokens.RepoKindTemplate,
			tokens.RepoKindInfra,
			tokens.RepoKindDocs,
		},
	}
	_, kind, err = kindPrompt.Run()
	if err != nil {
		return "", "", "", err
	}
	switch kind {
	case tokens.RepoKindService:
		app, err = functions.PromptForApp()
		if err != nil {
			return "", "", "", err
		}
	case tokens.RepoKindLibrary:
		scopePrompt := promptui.Select{
			Label: "Library scope",
			Items: []string{tokens.MenuLibGlobal, tokens.MenuTypeApp},
		}
		_, scope, err := scopePrompt.Run()
		if err != nil {
			return "", "", "", err
		}
		if scope == tokens.MenuTypeApp {
			app, err = functions.PromptForApp()
			if err != nil {
				return "", "", "", err
			}
		}
	case tokens.RepoKindTemplate:

		tkPrompt := promptui.Select{
			Label: "Template kind (what this template scaffolds)",
			Items: []string{
				tokens.TemplateKindService,
				tokens.TemplateKindLibrary,
				tokens.TemplateKindProject,
				tokens.TemplateKindInfra,
				tokens.TemplateKindDocs,
			},
		}
		_, templateKind, err = tkPrompt.Run()
		if err != nil {
			return "", "", "", err
		}
	}
	return kind, app, templateKind, nil
}

func runMenuAdopt(cmd *cobra.Command) error {
	urlPrompt := promptui.Prompt{
		Label: "Remote URL",
		Validate: func(input string) error {
			if strings.TrimSpace(input) == "" {
				return fmt.Errorf("remote URL is required")
			}
			return nil
		},
	}
	remoteURL, err := urlPrompt.Run()
	if err != nil {
		return err
	}

	kind, app, templateKind, err := promptForRepoKind()
	if err != nil {
		return err
	}

	namePrompt := promptui.Prompt{
		Label:   "Repo name (leave blank to derive from remote URL)",
		Default: "",
	}
	name, err := namePrompt.Run()
	if err != nil {
		return err
	}

	return functions.AdoptRepo(afero.NewOsFs(), cmd.OutOrStdout(), cmd.ErrOrStderr(), strings.TrimSpace(remoteURL), kind, strings.TrimSpace(name), app, templateKind)
}

func runMenuRegister(cmd *cobra.Command) error {
	kind, app, templateKind, err := promptForRepoKind()
	if err != nil {
		return err
	}

	namePrompt := promptui.Prompt{
		Label: "Repo name",
		Validate: func(input string) error {
			if strings.TrimSpace(input) == "" {
				return fmt.Errorf("repo name is required")
			}
			return nil
		},
	}
	name, err := namePrompt.Run()
	if err != nil {
		return err
	}

	remotePrompt := promptui.Prompt{
		Label:   "Remote URL (leave blank for None)",
		Default: "",
	}
	remote, err := remotePrompt.Run()
	if err != nil {
		return err
	}

	return functions.RegisterRepo(afero.NewOsFs(), cmd.OutOrStdout(), kind, strings.TrimSpace(name), app, strings.TrimSpace(remote), templateKind)
}

func runMenuTask(cmd *cobra.Command) error {
	namePrompt := promptui.Prompt{
		Label: "Workspace task name",
		Validate: func(input string) error {
			if strings.TrimSpace(input) == "" {
				return fmt.Errorf("task name is required")
			}
			return nil
		},
	}
	taskName, err := namePrompt.Run()
	if err != nil {
		return err
	}

	targetPrompt := promptui.Prompt{
		Label: "Target repo name",
		Validate: func(input string) error {
			if strings.TrimSpace(input) == "" {
				return fmt.Errorf("target is required")
			}
			return nil
		},
	}
	target, err := targetPrompt.Run()
	if err != nil {
		return err
	}

	appPrompt := promptui.Prompt{
		Label:   "Parent app (leave blank if the target is a project, global library, or app)",
		Default: "",
	}
	app, err := appPrompt.Run()
	if err != nil {
		return err
	}

	return functions.RunWorkspaceTask(afero.NewOsFs(), cmd.OutOrStdout(), cmd.ErrOrStderr(), strings.TrimSpace(taskName), strings.TrimSpace(target), strings.TrimSpace(app))
}
