package scaffold

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/apps"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/compose"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/services"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/workspace"
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

	templatePath := filepath.Join("repos", "templates", "apps")
	if _, err := fs.Stat(templatePath); err != nil {
		return fmt.Errorf("missing template: %w", err)
	}

	if err := CopyDir(fs, templatePath, appPath); err != nil {
		return err
	}
	return addAppToWorkspace(fs, name)
}

func AddService(fs afero.Fs, appName string, serviceName string, template string, dockerfile string, replacements map[string]string) error {
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
		return WireServiceToCompose(fs, appPath, appName, serviceName, dockerfile, "")
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
	return WireServiceToCompose(fs, appPath, appName, serviceName, dockerfile)
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

func WireServiceToCompose(fs afero.Fs, appPath string, appName string, serviceName string, dockerfile string) error {
	port, err := services.NextServicePort(fs, appName)
	if err != nil {
		return err
	}
	imageName, err := services.ServiceImageReference(fs, appName, serviceName)
	if err != nil {
		return err
	}
	if err := compose.AddServiceToCompose(fs, filepath.Join(appPath, "docker-compose.yml"), serviceName, map[string]any{
		"image": imageName,
		"ports": []string{fmt.Sprintf("%s:%s", port, port)},
	}); err != nil {
		return err
	}
	if err := compose.AddServiceToCompose(fs, filepath.Join(appPath, "docker-compose.dev.yml"), serviceName, map[string]any{
		"deploy": map[string]any{
			"resources": map[string]any{
				"limits": map[string]any{
					"cpus":   "0.50",
					"memory": "128M",
				},
				"reservations": map[string]any{
					"cpus":   "0.25",
					"memory": "64M",
				},
			},
		},
	}); err != nil {
		return err
	}
	image := services.DefaultServiceImage(appName, serviceName)
	return services.AddServiceToWorkspace(fs, appName, serviceName, port, image, dockerfile)
}

func addAppToWorkspace(fs afero.Fs, name string) error {
	config, err := workspace.ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	if apps.FindAppIndex(config, name) != -1 {
		return nil
	}
	config.Apps = append(config.Apps, workspace.WorkspaceApp{
		Name:      name,
		Services: []workspace.WorkspaceService{},
		Libraries: []workspace.WorkspaceLibrary{},
	})
	return workspace.WriteWorkspaceConfig(fs, config)
}

func removeAppFromWorkspace(fs afero.Fs, name string) error {
	if _, err := fs.Stat("ocean.workspace.json"); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	config, err := workspace.ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	if len(config.Apps) == 0 {
		return nil
	}
	updated := make([]workspace.WorkspaceApp, 0, len(config.Apps))
	removed := false
	for _, app := range config.Apps {
		if app.Name == name {
			removed = true
			continue
		}
		updated = append(updated, app)
	}
	if !removed {
		return nil
	}
	config.Apps = updated
	return workspace.WriteWorkspaceConfig(fs, config)
}
