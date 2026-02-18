package tree

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
)

type Component struct {
	Name string
	Path string
}

var appsDir string = "repos/apps"
var projectsDir string = "repos/projects"
var templateDir string = "repos/templates"
var libDir string = "repos/libs"
var sandboxDir string = "repos/sandbox"

func GetRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	curr := wd
	for {
		if _, err := os.Stat(filepath.Join(curr, "repos")); err == nil {
			return curr, nil
		}
		parent := filepath.Dir(curr)
		if parent == curr {
			return "", os.ErrNotExist
		}
		curr = parent
	}
}

func getSubfolders(relativePath string) ([]Component, error) {
	root, err := GetRoot()
	if err != nil {
		return nil, err
	}

	fullPath := filepath.Join(root, relativePath)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}

	var results []Component
	for _, entry := range entries {
		if entry.IsDir() {
			results = append(results, Component{
				Name: entry.Name(),
				Path: filepath.Join(fullPath, entry.Name()),
			})
		}
	}
	return results, nil
}

func GetApps() ([]Component, error) {
	return getSubfolders(appsDir)
}

func GetAppServices(appName string) ([]Component, error) {
	return getSubfolders(filepath.Join(appsDir, appName, "services"))
}

func GetAppClients(appName string) ([]Component, error) {
	return getSubfolders(filepath.Join(appsDir, appName, "clients"))
}

func GetAppLibs(appName string) ([]Component, error) {
	return getSubfolders(filepath.Join(appsDir, appName, "libs"))
}

func GetAppTests(appName string) ([]Component, error) {
	return getSubfolders(filepath.Join(appsDir, appName, "testing"))
}

func GetProjects() ([]Component, error) {
	return getSubfolders(projectsDir)
}

func GetLibs() ([]Component, error) {
	return getSubfolders(libDir)
}

func GetTemplates() ([]Component, error) {
	return getSubfolders(templateDir)
}

func IsSandboxEmpty() (bool, error) {
	root, err := GetRoot()
	if err != nil {
		return true, err
	}
	entries, err := os.ReadDir(filepath.Join(root, sandboxDir))
	if err != nil {
		return true, err
	}
	return len(entries) == 0, nil
}

func DirExists(fs afero.Fs, path string) bool {
	info, err := fs.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

type ProfileEntry struct {
	Name  string
	Label string
}

func ListApiTemplates() ([]string, error) {
	return ListTemplatesByType("service")
}

type templateConfig struct {
	Language string `json:"language"`
	Type     string `json:"type"`
}

func ListTemplatesByType(kind string) ([]string, error) {
	root, err := GetRoot()
	if err != nil {
		return nil, err
	}
	templateRoot := filepath.Join(root, "repos", "templates")
	entries, err := os.ReadDir(templateRoot)
	if err != nil {
		return nil, err
	}

	var templates []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		configPath := filepath.Join(templateRoot, entry.Name(), "ocean.config.json")
		payload, err := os.ReadFile(configPath)
		if err != nil {
			continue
		}
		var config templateConfig
		if err := json.Unmarshal(payload, &config); err != nil {
			continue
		}
		if strings.TrimSpace(config.Type) != kind {
			continue
		}
		templates = append(templates, entry.Name())
	}
	return templates, nil
}
