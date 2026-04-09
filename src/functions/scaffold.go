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

	"github.com/spf13/afero"
)

func CopyDir(fs afero.Fs, src string, dst string) error {
	return CopyDirWithReplacements(fs, src, dst, nil)
}

func CopyDirWithReplacements(fs afero.Fs, src string, dst string, replacements map[string]string) error {
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

		targetPath := filepath.Join(dst, replacePlaceholders(relPath, replacements))
		if info.IsDir() {
			return fs.MkdirAll(targetPath, 0o755)
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

// appSubdirs is the fixed list of subdirectories scaffolded under every new
// app. Apps are intentionally not template-able — Dusk Ocean creates this
// folder structure directly so the layout stays stable across the workspace.
var appSubdirs = []string{"services", "libs", "jobs", "docs", "testing"}

func AddApp(fs afero.Fs, name string) error {
	if name == "" {
		return fmt.Errorf("--name is required")
	}

	appPath := filepath.Join("repos", "apps", name)
	if _, err := fs.Stat(appPath); err == nil {
		return fmt.Errorf("app already exists: %s", name)
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := fs.MkdirAll(appPath, 0o755); err != nil {
		return err
	}
	for _, sub := range appSubdirs {
		subPath := filepath.Join(appPath, sub)
		if err := fs.MkdirAll(subPath, 0o755); err != nil {
			return err
		}
		if err := afero.WriteFile(fs, filepath.Join(subPath, ".gitkeep"), nil, 0o644); err != nil {
			return err
		}
	}

	appConfig := struct {
		Name     string `json:"name"`
		Language string `json:"language"`
		Type     string `json:"type"`
		Tasks    struct {
			Run string `json:"run"`
		} `json:"tasks"`
	}{
		Name:     name,
		Language: "",
		Type:     "app",
	}
	payload, err := json.MarshalIndent(appConfig, "", "    ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	if err := afero.WriteFile(fs, filepath.Join(appPath, "ocean.config.json"), payload, 0o644); err != nil {
		return err
	}

	return addAppToWorkspace(fs, name)
}

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

	if err := CopyDirWithReplacements(fs, templatePath, servicePath, replacements); err != nil {
		return err
	}
	return registerService(fs, appName, serviceName, dockerfile, containerFile)
}

var placeholderPattern = regexp.MustCompile(`{{\s*([^{}]+?)\s*}}`)

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

func registerService(fs afero.Fs, appName string, serviceName string, dockerfile string, containerFile string) error {
	port, err := NextServicePort(fs, appName)
	if err != nil {
		return err
	}
	image := DefaultServiceImage(appName, serviceName)
	return AddServiceToWorkspace(fs, appName, serviceName, port, image, dockerfile, containerFile)
}

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
