package functions

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
	"github.com/spf13/afero"
)

// CopyDir copies a directory tree verbatim, applying no placeholder substitution.
func CopyDir(fs afero.Fs, src string, dst string) error {
	return CopyDirWithReplacements(fs, src, dst, nil)
}

/*
CopyTemplate copies a scaffold template from src to dst, applying the
workspace's .oceanignore rules so directories like .git, node_modules, and
build artifacts don't propagate from a template into a newly scaffolded
entity. Use this for any `add` command that seeds from repos/templates/.
*/
func CopyTemplate(fs afero.Fs, src string, dst string, replacements map[string]string) error {
	return CopyDirWithReplacements(fs, src, dst, replacements)
}

// CopyDirWithReplacements copies a directory tree, honouring the workspace ignore rules and substituting placeholders in both path segments and file contents.
func CopyDirWithReplacements(fs afero.Fs, src string, dst string, replacements map[string]string) error {
	workspaceRoot, _ := GetRoot()
	var ignorePatterns []string
	if workspaceRoot != "" {
		ignorePatterns, _ = ReadOceanIgnorePatterns(fs, workspaceRoot)
	}

	return afero.Walk(fs, src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return fs.MkdirAll(dst, 0o755)
		}

		if ShouldIgnore(filepath.ToSlash(relPath), info.IsDir(), ignorePatterns) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		targetPath := filepath.Join(dst, replacePlaceholders(relPath, replacements))
		if info.IsDir() {
			return fs.MkdirAll(targetPath, 0o755)
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		if err := fs.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}

		if len(replacements) == 0 {
			sourceFile, err := fs.Open(path)
			if err != nil {
				return err
			}
			defer sourceFile.Close()

			targetFile, err := fs.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
			if err != nil {
				return err
			}
			defer targetFile.Close()

			_, err = io.Copy(targetFile, sourceFile)
			return err
		}

		content, err := afero.ReadFile(fs, path)
		if err != nil {
			return err
		}
		updated := replacePlaceholders(string(content), replacements)
		return afero.WriteFile(fs, targetPath, []byte(updated), info.Mode().Perm())
	})
}

var appSubdirs = []string{
	tokens.AppSubDirServices,
	tokens.AppSubDirLibs,
	tokens.AppSubDirProjects,
	tokens.AppSubDirJobsDocker,
	tokens.AppSubDirJobsMigration,
	tokens.AppSubDirJobsScripts,
	tokens.AppSubDirTesting,
}

/*
AddApp scaffolds repos/apps/<name> from a composite app template, backfills any
canonical subdirectory the template omitted, and registers the app in workspace
config. A template is required: unlike a service or a non-code repo, an app has
no boilerplate form.
*/
func AddApp(fs afero.Fs, name string, template string, replacements map[string]string) error {
	if name == "" {
		return fmt.Errorf("--name is required")
	}
	if strings.TrimSpace(template) == "" {
		return fmt.Errorf("--template is required: apps scaffold from an app template")
	}

	appPath := filepath.Join("repos", tokens.RepoDirApps, name)
	if _, err := fs.Stat(appPath); err == nil {
		return fmt.Errorf("app already exists: %s", name)
	} else if !os.IsNotExist(err) {
		return err
	}

	templatePath := filepath.Join("repos", tokens.RepoDirTemplates, template)
	if _, err := fs.Stat(templatePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("missing template: %s", template)
		}
		return err
	}

	if err := CopyTemplate(fs, templatePath, appPath, replacements); err != nil {
		return err
	}

	if err := backfillAppSubdirs(fs, appPath); err != nil {
		return err
	}

	configPath := filepath.Join(appPath, "ocean.config.json")
	if _, err := fs.Stat(configPath); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := WriteStarterRepoConfig(fs, appPath, name, tokens.RepoKindApp); err != nil {
			return err
		}
	}

	return addAppToWorkspace(fs, name)
}

/*
backfillAppSubdirs creates each canonical app subdirectory the template did not
ship, marking only the ones left empty with a .gitkeep so a populated directory
is never polluted.
*/
func backfillAppSubdirs(fs afero.Fs, appPath string) error {
	for _, sub := range appSubdirs {
		subPath := filepath.Join(appPath, filepath.FromSlash(sub))
		if err := fs.MkdirAll(subPath, 0o755); err != nil {
			return err
		}
		entries, err := afero.ReadDir(fs, subPath)
		if err != nil {
			return err
		}
		if len(entries) > 0 {
			continue
		}
		if err := afero.WriteFile(fs, filepath.Join(subPath, ".gitkeep"), nil, 0o644); err != nil {
			return err
		}
	}
	return nil
}

/*
RunAppSetupTasks runs the setup task for an app and for every nested unit its
template shipped, in discovery order, so a composite scaffold finishes with each
unit's toolchain initialized. It aborts on the first failure rather than leaving
a half-initialized tree behind.
*/
func RunAppSetupTasks(fs afero.Fs, root string, appName string, stdout io.Writer, stderr io.Writer) error {
	appPath := filepath.Join("repos", tokens.RepoDirApps, appName)
	if err := RunSetupTask(fs, appPath, root, stdout, stderr); err != nil {
		return err
	}
	discovered, err := DiscoverAppSubRepos(fs, appName)
	if err != nil {
		return err
	}
	for _, sub := range discovered {
		if err := RunSetupTask(fs, sub.Path, root, stdout, stderr); err != nil {
			return fmt.Errorf("setup failed for %s %s/%s: %w", sub.Kind, appName, sub.Name, err)
		}
	}
	return nil
}

// AddService scaffolds a service under an app, seeding from a template when one is given and registering it with the next free port.
func AddService(fs afero.Fs, appName string, serviceName string, template string, dockerfile string, containerFile string, replacements map[string]string) error {
	appPath := filepath.Join("repos", "apps", appName)
	if _, err := fs.Stat(appPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("app does not exist: %s", appName)
		}
		return err
	}

	servicePath := filepath.Join(appPath, "services", serviceName)
	if _, err := fs.Stat(servicePath); err == nil {
		return fmt.Errorf("service already exists: %s", serviceName)
	} else if !os.IsNotExist(err) {
		return err
	}

	if strings.TrimSpace(template) == "" {
		if err := fs.MkdirAll(servicePath, 0o755); err != nil {
			return err
		}
		config := struct {
			Name     string `json:"name"`
			Language string `json:"language"`
			Type     string `json:"type"`
			Tasks    struct {
				Build string `json:"build"`
				Test  string `json:"test"`
			} `json:"tasks"`
		}{
			Name:     serviceName,
			Language: "",
			Type:     "service",
			Tasks: struct {
				Build string `json:"build"`
				Test  string `json:"test"`
			}{
				Build: "",
				Test:  "",
			},
		}
		payload, err := json.MarshalIndent(config, "", "    ")
		if err != nil {
			return err
		}
		payload = append(payload, '\n')
		if err := afero.WriteFile(fs, filepath.Join(servicePath, "ocean.config.json"), payload, 0o644); err != nil {
			return err
		}
		return registerService(fs, appName, serviceName, dockerfile, containerFile)
	}

	templatePath := filepath.Join("repos", "templates", template)
	if _, err := fs.Stat(templatePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("missing template: %s", template)
		}
		return err
	}

	if err := CopyTemplate(fs, templatePath, servicePath, replacements); err != nil {
		return err
	}
	return registerService(fs, appName, serviceName, dockerfile, containerFile)
}

var placeholderPattern = regexp.MustCompile(`{{\s*([^{}]+?)\s*}}`)

// replacePlaceholders substitutes every known placeholder token in a string, leaving unknown ones untouched.
func replacePlaceholders(value string, replacements map[string]string) string {
	if len(replacements) == 0 {
		return value
	}
	return placeholderPattern.ReplaceAllStringFunc(value, func(match string) string {
		parts := placeholderPattern.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		key := strings.TrimSpace(parts[1])
		replacement, ok := replacements[key]
		if !ok {
			return match
		}
		return replacement
	})
}

// AddInfra scaffolds an infrastructure repo, seeding from a template when one is given.
func AddInfra(fs afero.Fs, name string, template string, replacements map[string]string) error {
	return addNonCodeRepo(fs, tokens.RepoKindInfra, name, template, replacements, AddInfraToWorkspace)
}

// AddDocs scaffolds a docs repo, seeding from a template when one is given.
func AddDocs(fs afero.Fs, name string, template string, replacements map[string]string) error {
	return addNonCodeRepo(fs, tokens.RepoKindDocs, name, template, replacements, AddDocsToWorkspace)
}

// RemoveInfra deletes an infrastructure repo and unregisters it.
func RemoveInfra(fs afero.Fs, name string) error {
	return removeNonCodeRepo(fs, tokens.RepoKindInfra, name, RemoveInfraFromWorkspace)
}

// RemoveDocs deletes a docs repo and unregisters it.
func RemoveDocs(fs afero.Fs, name string) error {
	return removeNonCodeRepo(fs, tokens.RepoKindDocs, name, RemoveDocsFromWorkspace)
}

// addNonCodeRepo scaffolds an infra or docs repo, writing a starter config when the template shipped none, then registers it.
func addNonCodeRepo(fs afero.Fs, kind string, name string, template string, replacements map[string]string, register func(afero.Fs, string) error) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("--name is required")
	}
	relPath, err := ResolveRepoPath(kind, name, "")
	if err != nil {
		return err
	}
	if _, err := fs.Stat(relPath); err == nil {
		return fmt.Errorf("%s already exists: %s", kind, name)
	} else if !os.IsNotExist(err) {
		return err
	}

	if strings.TrimSpace(template) == "" {
		if err := fs.MkdirAll(relPath, 0o755); err != nil {
			return err
		}
		if err := WriteStarterRepoConfig(fs, relPath, name, kind); err != nil {
			return err
		}
		return register(fs, name)
	}

	templatePath := filepath.Join("repos", tokens.RepoDirTemplates, template)
	if _, err := fs.Stat(templatePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("missing template: %s", template)
		}
		return err
	}
	if err := CopyDirWithReplacements(fs, templatePath, relPath, replacements); err != nil {
		return err
	}

	configPath := filepath.Join(relPath, "ocean.config.json")
	if _, err := fs.Stat(configPath); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := WriteStarterRepoConfig(fs, relPath, name, kind); err != nil {
			return err
		}
	}
	return register(fs, name)
}

// removeNonCodeRepo deletes an infra or docs repo directory and unregisters it.
func removeNonCodeRepo(fs afero.Fs, kind string, name string, unregister func(afero.Fs, string) error) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("--name is required")
	}
	relPath, err := ResolveRepoPath(kind, name, "")
	if err != nil {
		return err
	}
	if info, err := fs.Stat(relPath); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("%s exists and is not a directory", relPath)
		}
		if err := fs.RemoveAll(relPath); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return unregister(fs, name)
}

// RemoveApp deletes an app directory and unregisters it along with everything nested inside.
func RemoveApp(fs afero.Fs, name string) error {
	if name == "" {
		return fmt.Errorf("--name is required")
	}

	appPath := filepath.Join("repos", "apps", name)
	if _, err := fs.Stat(appPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("app does not exist: %s", name)
		}
		return err
	}

	if err := fs.RemoveAll(appPath); err != nil {
		return err
	}
	return removeAppFromWorkspace(fs, name)
}

// registerService records a new service in workspace config with the next free port and the default image name.
func registerService(fs afero.Fs, appName string, serviceName string, dockerfile string, containerFile string) error {
	port, err := NextServicePort(fs, appName)
	if err != nil {
		return err
	}
	image := DefaultServiceImage(appName, serviceName)
	return AddServiceToWorkspace(fs, appName, serviceName, port, image, dockerfile, containerFile)
}

// addAppToWorkspace records a new app in workspace config with empty unit lists, no-op when it is already present.
func addAppToWorkspace(fs afero.Fs, name string) error {
	config, err := ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	if FindAppIndex(config, name) != -1 {
		return nil
	}
	config.Apps = append(config.Apps, WorkspaceApp{
		Name:      name,
		Services:  []WorkspaceService{},
		Libraries: []WorkspaceLibrary{},
		Testing:   []WorkspaceTest{},
	})
	return WriteWorkspaceConfig(fs, config)
}

// RunSetupTask runs a repo's setup task if it declares one, no-op when the repo has no config or no such task.
func RunSetupTask(fs afero.Fs, repoPath string, root string, stdout io.Writer, stderr io.Writer) error {
	config, err := ReadRepoConfig(fs, repoPath)
	if err != nil {
		return nil
	}
	setupCmd, err := RepoCommand(config, "setup")
	if err != nil || strings.TrimSpace(setupCmd) == "" {
		return nil
	}
	cmd := exec.Command("sh", "-c", setupCmd)
	cmd.Dir = filepath.Join(root, repoPath)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// removeAppFromWorkspace drops an app's entry from workspace config.
func removeAppFromWorkspace(fs afero.Fs, name string) error {
	if _, err := fs.Stat("ocean.workspace.json"); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return UpdateConfig(fs, func(config WorkspaceConfig) (WorkspaceConfig, error) {
		if len(config.Apps) == 0 {
			return config, nil
		}
		updated := make([]WorkspaceApp, 0, len(config.Apps))
		removed := false
		for _, app := range config.Apps {
			if app.Name == name {
				removed = true
				continue
			}
			updated = append(updated, app)
		}
		if !removed {
			return config, nil
		}
		config.Apps = updated
		return config, nil
	})
}
