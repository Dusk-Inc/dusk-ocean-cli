package functions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/models"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// Type aliases re-exported from models so callers using the functions package
// continue to work without changes.
type (
	WorkspaceConfig       = models.WorkspaceConfig
	WorkspacePorts        = models.WorkspacePorts
	WorkspacePortRange    = models.WorkspacePortRange
	WorkspaceReservedPort = models.WorkspaceReservedPort
	WorkspaceApp          = models.WorkspaceApp
	WorkspaceImage        = models.WorkspaceImage
	WorkspaceService      = models.WorkspaceService
	WorkspaceLibrary      = models.WorkspaceLibrary
	WorkspaceProject      = models.WorkspaceProject
	WorkspaceTest         = models.WorkspaceTest
	WorkspaceTemplate     = models.WorkspaceTemplate
	WorkspaceDep          = models.WorkspaceDep
	RepoConfig            = models.RepoConfig
	TargetKind            = models.TargetKind
	Target                = models.Target
	InitOptions           = models.InitOptions
)

const (
	TargetApp       = models.TargetApp
	TargetService   = models.TargetService
	TargetAppLib    = models.TargetAppLib
	TargetGlobalLib = models.TargetGlobalLib
	TargetProject   = models.TargetProject
	TargetTest      = models.TargetTest
	TargetTemplate  = models.TargetTemplate
)

const defaultImageTag = "dev"

func EnsureWorkspaceRoot(fs afero.Fs) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	root, err := GetRoot()
	if err != nil {
		return "", err
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return "", err
	}
	if absRoot != absCwd {
		return "", fmt.Errorf("command must be run from the workspace root (%s)", absRoot)
	}
	if _, err := fs.Stat("ocean.workspace.json"); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("workspace config not found: ocean.workspace.json")
		}
		return "", err
	}
	return absRoot, nil
}


func MakeConfig(libs []WorkspaceLibrary, apps []WorkspaceApp, projects []WorkspaceProject) WorkspaceConfig {
	return WorkspaceConfig{
		Workspace: "test",
		Apps:      apps,
		Libraries: libs,
		Projects:  projects,
	}
}

func MakeLibrary(name string, deps ...string) WorkspaceLibrary {
	return WorkspaceLibrary{
		Name: name,
		Deps: makeGlobalDeps(deps...),
	}
}

func MakeProject(name string, deps ...string) WorkspaceProject {
	return WorkspaceProject{
		Name: name,
		Deps: makeGlobalDeps(deps...),
	}
}

func MakeApp(name string, libraries []WorkspaceLibrary) WorkspaceApp {
	return WorkspaceApp{
		Name:      name,
		Services:  nil,
		Libraries: libraries,
		Testing:   nil,
	}
}

func MakeTemplate(name string, kind string, deps ...string) WorkspaceTemplate {
	return WorkspaceTemplate{
		Name: name,
		Kind: kind,
		Deps: makeGlobalDeps(deps...),
	}
}

func InitWorkspace(fs afero.Fs, out io.Writer, opts InitOptions) error {
	if opts.Name == "" {
		return fmt.Errorf("--name is required")
	}
	if err := ensureWorkspaceFile(fs, out, opts); err != nil {
		return err
	}

	dirs := []string{
		".ocean",
		filepath.Join(".ocean", "results"),
		filepath.Join(".ocean", "hashes"),
		"repos",
		filepath.Join("repos", "apps"),
		filepath.Join("repos", "libs"),
		filepath.Join("repos", "projects"),
		filepath.Join("repos", "templates"),
		filepath.Join("repos", "containers"),
	}

	for _, dir := range dirs {
		created, err := ensureDir(fs, out, dir)
		if err != nil {
			return err
		}
		if created {
			if shouldCreateGitkeep(dir) {
				if err := ensureGitkeep(fs, dir); err != nil {
					return err
				}
			}
		}
	}

	return ensureGitIgnore(fs)
}

func ensureWorkspaceFile(fs afero.Fs, out io.Writer, opts InitOptions) error {
	path := "ocean.workspace.json"
	if _, err := fs.Stat(path); err == nil {
		fmt.Fprintf(out, "already exists: %s\n", path)
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	tasks := map[string]string{}
	for _, name := range tokens.DefaultWorkspaceTaskNames {
		tasks[name] = ""
	}

	workspaceConfig := WorkspaceConfig{
		Workspace: opts.Name,
		Version:   "",
		Variables: map[string]string{},
		Tasks:     tasks,
		Ports: WorkspacePorts{
			Allowed: WorkspacePortRange{
				Min: 3000,
				Max: 3999,
			},
			Reserved: []WorkspaceReservedPort{},
		},
		Apps:      []WorkspaceApp{},
		Libraries: []WorkspaceLibrary{},
		Projects:  []WorkspaceProject{},
		Templates: []WorkspaceTemplate{},
	}

	payload, err := json.MarshalIndent(workspaceConfig, "", "    ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	return afero.WriteFile(fs, path, payload, 0o644)
}

func ensureDir(fs afero.Fs, out io.Writer, path string) (bool, error) {
	info, err := fs.Stat(path)
	if err == nil {
		if !info.IsDir() {
			return false, fmt.Errorf("%s exists and is not a directory", path)
		}
		fmt.Fprintf(out, "already exists: %s\n", path)
		return false, nil
	}
	if !os.IsNotExist(err) {
		return false, err
	}
	if err := fs.MkdirAll(path, 0o755); err != nil {
		return false, err
	}
	return true, nil
}

func ensureGitkeep(fs afero.Fs, dir string) error {
	path := filepath.Join(dir, ".gitkeep")
	if _, err := fs.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	file, err := fs.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	return file.Close()
}

func shouldCreateGitkeep(dir string) bool {
	switch dir {
	case ".ocean", filepath.Join(".ocean", "results"), filepath.Join(".ocean", "hashes"):
		return false
	default:
		return true
	}
}

func ensureFile(fs afero.Fs, out io.Writer, path string, contents string) error {
	if _, err := fs.Stat(path); err == nil {
		fmt.Fprintf(out, "already exists: %s\n", path)
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := fs.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return afero.WriteFile(fs, path, []byte(contents), 0o644)
}

func ensureGitIgnore(fs afero.Fs) error {
	path := ".gitignore"
	entry := ".ocean"

	existing, err := afero.ReadFile(fs, path)
	if err != nil {
		if os.IsNotExist(err) {
			content := entry + "\n"
			return afero.WriteFile(fs, path, []byte(content), 0o644)
		}
		return err
	}

	content := string(existing)
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	for _, line := range strings.Split(normalized, "\n") {
		if strings.TrimSpace(line) == entry {
			return nil
		}
	}

	if len(content) > 0 && !strings.HasSuffix(normalized, "\n") {
		normalized += "\n"
	}
	normalized += entry + "\n"
	return afero.WriteFile(fs, path, []byte(normalized), 0o644)
}

func ReadWorkspaceConfig(fs afero.Fs) (WorkspaceConfig, error) {
	var config WorkspaceConfig
	content, err := afero.ReadFile(fs, "ocean.workspace.json")
	if err != nil {
		if os.IsNotExist(err) {
			return WorkspaceConfig{}, fmt.Errorf("workspace config not found: ocean.workspace.json")
		}
		return WorkspaceConfig{}, err
	}
	if err := json.Unmarshal(content, &config); err != nil {
		return WorkspaceConfig{}, err
	}
	normalized := normalizeWorkspaceConfig(config)
	if err := ValidateWorkspaceConfig(normalized); err != nil {
		return WorkspaceConfig{}, err
	}
	return normalized, nil
}

func ReadGitignorePatterns(fs afero.Fs, root string) ([]string, error) {
	path := filepath.Join(root, ".gitignore")
	payload, err := afero.ReadFile(fs, path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	normalized := strings.ReplaceAll(string(payload), "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	patterns := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		patterns = append(patterns, trimmed)
	}
	return patterns, nil
}

// CollectRepoIgnorePatterns returns the workspace-root .gitignore patterns
// plus the repo-local .gitignore patterns, in that order. This ensures that
// repo-scoped ignores (e.g. `dist/`, `coverage/`) are honored when hashing a
// repo directory — without which every rebuild produces a new dependency-tree
// hash and defeats Ocean's skip cache. Patterns with leading `/` in the repo's
// .gitignore are rooted at the repo directory, which matches the walk origin
// in CalcDirHash. Nested .gitignore files below the repo root are not yet
// consulted.
func CollectRepoIgnorePatterns(fs afero.Fs, workspaceRoot string, repoPath string) ([]string, error) {
	workspacePatterns, err := ReadGitignorePatterns(fs, workspaceRoot)
	if err != nil {
		return nil, err
	}
	if repoPath == "" || repoPath == workspaceRoot {
		return workspacePatterns, nil
	}
	repoPatterns, err := ReadGitignorePatterns(fs, repoPath)
	if err != nil {
		return nil, err
	}
	if len(repoPatterns) == 0 {
		return workspacePatterns, nil
	}
	merged := make([]string, 0, len(workspacePatterns)+len(repoPatterns))
	merged = append(merged, workspacePatterns...)
	merged = append(merged, repoPatterns...)
	return merged, nil
}

// UpdateConfig reads the workspace config, applies a mutation, and writes it back if changed.
func UpdateConfig(fs afero.Fs, update func(WorkspaceConfig) (WorkspaceConfig, error)) error {
	config, err := ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	original, err := json.Marshal(config)
	if err != nil {
		return err
	}
	updated, err := update(config)
	if err != nil {
		return err
	}
	normalized, err := json.Marshal(updated)
	if err != nil {
		return err
	}
	if bytes.Equal(original, normalized) {
		return nil
	}
	return WriteWorkspaceConfig(fs, updated)
}

func WriteWorkspaceConfig(fs afero.Fs, config WorkspaceConfig) error {
	normalized := normalizeWorkspaceConfig(config)
	if err := ValidateWorkspaceConfig(normalized); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(normalized, "", "    ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	return afero.WriteFile(fs, "ocean.workspace.json", payload, 0o644)
}

func normalizeWorkspaceConfig(config WorkspaceConfig) WorkspaceConfig {
	if config.Apps == nil {
		config.Apps = []WorkspaceApp{}
	}
	if config.Libraries == nil {
		config.Libraries = []WorkspaceLibrary{}
	}
	if config.Projects == nil {
		config.Projects = []WorkspaceProject{}
	}
	if config.Ports.Allowed.Min == 0 {
		config.Ports.Allowed.Min = 3000
	}
	if config.Ports.Allowed.Max == 0 {
		config.Ports.Allowed.Max = 3999
	}
	if config.Ports.Reserved == nil {
		config.Ports.Reserved = []WorkspaceReservedPort{}
	}
	for i := range config.Apps {
		if config.Apps[i].Services == nil {
			config.Apps[i].Services = []WorkspaceService{}
		}
		if config.Apps[i].Libraries == nil {
			config.Apps[i].Libraries = []WorkspaceLibrary{}
		}
		for j := range config.Apps[i].Services {
			if config.Apps[i].Services[j].Scopes == nil {
				config.Apps[i].Services[j].Scopes = []string{}
			}
			if config.Apps[i].Services[j].Deps == nil {
				config.Apps[i].Services[j].Deps = []WorkspaceDep{}
			}
			if config.Apps[i].Services[j].Image.Name == "" {
				config.Apps[i].Services[j].Image.Name = fmt.Sprintf("%s__%s", config.Apps[i].Name, config.Apps[i].Services[j].Name)
			}
			if config.Apps[i].Services[j].Image.Tag == "" {
				config.Apps[i].Services[j].Image.Tag = defaultImageTag
			}
		}
		for j := range config.Apps[i].Libraries {
			if config.Apps[i].Libraries[j].Scopes == nil {
				config.Apps[i].Libraries[j].Scopes = []string{}
			}
			if config.Apps[i].Libraries[j].Deps == nil {
				config.Apps[i].Libraries[j].Deps = []WorkspaceDep{}
			}
		}
		if config.Apps[i].Testing == nil {
			config.Apps[i].Testing = []WorkspaceTest{}
		}
		for j := range config.Apps[i].Testing {
			if config.Apps[i].Testing[j].Scopes == nil {
				config.Apps[i].Testing[j].Scopes = []string{}
			}
			if config.Apps[i].Testing[j].Deps == nil {
				config.Apps[i].Testing[j].Deps = []WorkspaceDep{}
			}
		}
	}
	for i := range config.Libraries {
		if config.Libraries[i].Scopes == nil {
			config.Libraries[i].Scopes = []string{}
		}
		if config.Libraries[i].Deps == nil {
			config.Libraries[i].Deps = []WorkspaceDep{}
		}
	}
	for i := range config.Projects {
		if config.Projects[i].Scopes == nil {
			config.Projects[i].Scopes = []string{}
		}
		if config.Projects[i].Deps == nil {
			config.Projects[i].Deps = []WorkspaceDep{}
		}
	}
	return config
}

// FindAppIndex returns the index of an app name, or -1 if not found.
func FindAppIndex(config WorkspaceConfig, appName string) int {
	for i, app := range config.Apps {
		if app.Name == appName {
			return i
		}
	}
	return -1
}

// FindServiceIndex returns the index of a service name within an app, or -1 if not found.
func FindServiceIndex(app WorkspaceApp, serviceName string) int {
	for i, service := range app.Services {
		if service.Name == serviceName {
			return i
		}
	}
	return -1
}

// FindAppLibraryIndex returns the index of an app library name within an app, or -1 if not found.
func FindAppLibraryIndex(app WorkspaceApp, libName string) int {
	for i, lib := range app.Libraries {
		if lib.Name == libName {
			return i
		}
	}
	return -1
}

// FindAppTestIndex returns the index of an app test name within an app, or -1 if not found.
func FindAppTestIndex(app WorkspaceApp, testName string) int {
	for i, test := range app.Testing {
		if test.Name == testName {
			return i
		}
	}
	return -1
}

// FindGlobalLibraryIndex returns the index of a global library name, or -1 if not found.
func FindGlobalLibraryIndex(config WorkspaceConfig, libName string) int {
	for i, lib := range config.Libraries {
		if lib.Name == libName {
			return i
		}
	}
	return -1
}

// FindProjectIndex returns the index of a project name, or -1 if not found.
func FindProjectIndex(config WorkspaceConfig, projectName string) int {
	for i, project := range config.Projects {
		if project.Name == projectName {
			return i
		}
	}
	return -1
}

// FindAppLibraryByName returns an app library by name within an app.
func FindAppLibraryByName(config WorkspaceConfig, appName string, name string) (WorkspaceLibrary, bool) {
	appIndex := FindAppIndex(config, appName)
	if appIndex == -1 {
		return WorkspaceLibrary{}, false
	}
	for _, lib := range config.Apps[appIndex].Libraries {
		if lib.Name == name {
			return lib, true
		}
	}
	return WorkspaceLibrary{}, false
}

// FindGlobalLibraryByName returns a global library by name and errors on ambiguity.
func FindGlobalLibraryByName(config WorkspaceConfig, name string) (WorkspaceLibrary, bool, error) {
	var match *WorkspaceLibrary
	for i, lib := range config.Libraries {
		if lib.Name != name {
			continue
		}
		if match != nil {
			return WorkspaceLibrary{}, false, fmt.Errorf("global library name is ambiguous: %s", name)
		}
		match = &config.Libraries[i]
	}
	if match == nil {
		return WorkspaceLibrary{}, false, nil
	}
	return *match, true, nil
}

// FindProjectByName returns a project by name and errors on ambiguity.
func FindProjectByName(config WorkspaceConfig, name string) (WorkspaceProject, bool, error) {
	var match *WorkspaceProject
	for i, project := range config.Projects {
		if project.Name != name {
			continue
		}
		if match != nil {
			return WorkspaceProject{}, false, fmt.Errorf("project name is ambiguous: %s", name)
		}
		match = &config.Projects[i]
	}
	if match == nil {
		return WorkspaceProject{}, false, nil
	}
	return *match, true, nil
}

// AddTestToWorkspace registers an app-scoped test project.
func AddTestToWorkspace(fs afero.Fs, appName string, name string) error {
	config, err := ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	appIndex := FindAppIndex(config, appName)
	if appIndex == -1 {
		config.Apps = append(config.Apps, WorkspaceApp{
			Name:      appName,
			Services:  []WorkspaceService{},
			Libraries: []WorkspaceLibrary{},
			Testing: []WorkspaceTest{
				{
					Name: name,
					Deps: []WorkspaceDep{},
				},
			},
		})
		return WriteWorkspaceConfig(fs, config)
	}
	if FindAppTestIndex(config.Apps[appIndex], name) != -1 {
		return nil
	}
	config.Apps[appIndex].Testing = append(config.Apps[appIndex].Testing, WorkspaceTest{
		Name: name,
		Deps: []WorkspaceDep{},
	})
	return WriteWorkspaceConfig(fs, config)
}

// RemoveTestFromWorkspace removes an app-scoped test project registration.
func RemoveTestFromWorkspace(fs afero.Fs, appName string, name string) error {
	return UpdateConfig(fs, func(config WorkspaceConfig) (WorkspaceConfig, error) {
		appIndex := FindAppIndex(config, appName)
		if appIndex == -1 {
			return config, nil
		}
		testIndex := FindAppTestIndex(config.Apps[appIndex], name)
		if testIndex == -1 {
			return config, nil
		}
		tests := config.Apps[appIndex].Testing
		config.Apps[appIndex].Testing = append(tests[:testIndex], tests[testIndex+1:]...)
		return config, nil
	})
}

// FindRepoLanguage returns whether the repo exists and its language if configured.
func FindRepoLanguage(fs afero.Fs, repoPath string) (bool, string, error) {
	info, err := fs.Stat(repoPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, "", nil
		}
		return false, "", err
	}
	if !info.IsDir() {
		return false, "", nil
	}
	config, err := ReadRepoConfig(fs, repoPath)
	if err != nil {
		return true, "", nil
	}
	return true, config.Language, nil
}

// AppNamesWithLibraries returns app names that contain libraries.
func AppNamesWithLibraries(config WorkspaceConfig) []string {
	names := []string{}
	for _, app := range config.Apps {
		if len(app.Libraries) == 0 {
			continue
		}
		names = append(names, app.Name)
	}
	return names
}

// AppNamesWithServices returns app names that contain services.
func AppNamesWithServices(config WorkspaceConfig) []string {
	names := []string{}
	for _, app := range config.Apps {
		if len(app.Services) == 0 {
			continue
		}
		names = append(names, app.Name)
	}
	return names
}

// AppNamesWithTests returns app names that contain testing projects.
func AppNamesWithTests(config WorkspaceConfig) []string {
	names := []string{}
	for _, app := range config.Apps {
		if len(app.Testing) == 0 {
			continue
		}
		names = append(names, app.Name)
	}
	return names
}

// AppLibraryNames returns library names for a given app.
func AppLibraryNames(config WorkspaceConfig, appName string) []string {
	for _, app := range config.Apps {
		if app.Name != appName {
			continue
		}
		names := make([]string, 0, len(app.Libraries))
		for _, lib := range app.Libraries {
			names = append(names, lib.Name)
		}
		return names
	}
	return nil
}

// ServiceNames returns service names for a given app.
func ServiceNames(config WorkspaceConfig, appName string) []string {
	for _, app := range config.Apps {
		if app.Name != appName {
			continue
		}
		names := make([]string, 0, len(app.Services))
		for _, service := range app.Services {
			names = append(names, service.Name)
		}
		return names
	}
	return nil
}

// TestNames returns testing project names for a given app.
func TestNames(config WorkspaceConfig, appName string) []string {
	for _, app := range config.Apps {
		if app.Name != appName {
			continue
		}
		names := make([]string, 0, len(app.Testing))
		for _, test := range app.Testing {
			names = append(names, test.Name)
		}
		return names
	}
	return nil
}

// GlobalLibraryNames returns names of global libraries.
func GlobalLibraryNames(config WorkspaceConfig) []string {
	names := make([]string, 0, len(config.Libraries))
	for _, lib := range config.Libraries {
		names = append(names, lib.Name)
	}
	return names
}

// ProjectNames returns names of projects.
func ProjectNames(config WorkspaceConfig) []string {
	names := make([]string, 0, len(config.Projects))
	for _, project := range config.Projects {
		names = append(names, project.Name)
	}
	return names
}

// TemplateNames returns names of all registered templates.
func TemplateNames(config WorkspaceConfig) []string {
	names := make([]string, 0, len(config.Templates))
	for _, template := range config.Templates {
		names = append(names, template.Name)
	}
	return names
}

// ResolveTargetByName resolves a workspace target by its repo name across all entity types.
// Returns an error if no match is found or if the name is ambiguous.
func ResolveTargetByName(config WorkspaceConfig, root string, name string) (Target, error) {
	var matches []Target

	for _, lib := range config.Libraries {
		if lib.Name == name {
			matches = append(matches, Target{
				Kind: TargetGlobalLib,
				Name: name,
				Path: filepath.Join(root, "repos", "libs", name),
			})
		}
	}

	for _, project := range config.Projects {
		if project.Name == name {
			matches = append(matches, Target{
				Kind: TargetProject,
				Name: name,
				Path: filepath.Join(root, "repos", "projects", name),
			})
		}
	}

	for _, template := range config.Templates {
		if template.Name == name {
			matches = append(matches, Target{
				Kind: TargetTemplate,
				Name: name,
				Path: filepath.Join(root, "repos", "templates", name),
			})
		}
	}

	for _, app := range config.Apps {
		if app.Name == name {
			matches = append(matches, Target{
				Kind: TargetApp,
				Name: name,
				Path: filepath.Join(root, "repos", "apps", name),
			})
		}
		for _, service := range app.Services {
			if service.Name == name {
				matches = append(matches, Target{
					Kind: TargetService,
					App:  app.Name,
					Name: name,
					Path: filepath.Join(root, "repos", "apps", app.Name, "services", name),
				})
			}
		}
		for _, lib := range app.Libraries {
			if lib.Name == name {
				matches = append(matches, Target{
					Kind: TargetAppLib,
					App:  app.Name,
					Name: name,
					Path: filepath.Join(root, "repos", "apps", app.Name, "libs", name),
				})
			}
		}
		for _, test := range app.Testing {
			if test.Name == name {
				matches = append(matches, Target{
					Kind: TargetTest,
					App:  app.Name,
					Name: name,
					Path: filepath.Join(root, "repos", "apps", app.Name, "testing", name),
				})
			}
		}
	}

	if len(matches) == 0 {
		return Target{}, fmt.Errorf("target not found: %s", name)
	}
	if len(matches) > 1 {
		return Target{}, fmt.Errorf("target name is ambiguous: %s (found in multiple repos)", name)
	}
	return matches[0], nil
}

// ResolveTarget resolves a workspace target from a path.
func ResolveTarget(fs afero.Fs, root string, cwd string) (Target, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Target{}, err
	}
	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return Target{}, err
	}
	rel, err := filepath.Rel(absRoot, absCwd)
	if err != nil {
		return Target{}, err
	}
	parts := strings.Split(filepath.Clean(rel), string(filepath.Separator))
	if len(parts) < 4 || parts[0] != "repos" {
		return Target{}, fmt.Errorf("current directory is not a valid install target")
	}

	switch parts[1] {
	case "apps":
		if len(parts) >= 5 && parts[3] == "services" {
			targetPath := filepath.Join(absRoot, "repos", "apps", parts[2], "services", parts[4])
			if !DirExists(fs, targetPath) {
				return Target{}, fmt.Errorf("service path does not exist: %s", targetPath)
			}
			return Target{
				Kind: TargetService,
				App:  parts[2],
				Name: parts[4],
				Path: targetPath,
			}, nil
		}
		if len(parts) >= 5 && parts[3] == "libs" {
			targetPath := filepath.Join(absRoot, "repos", "apps", parts[2], "libs", parts[4])
			if !DirExists(fs, targetPath) {
				return Target{}, fmt.Errorf("library path does not exist: %s", targetPath)
			}
			return Target{
				Kind: TargetAppLib,
				App:  parts[2],
				Name: parts[4],
				Path: targetPath,
			}, nil
		}
		if len(parts) >= 5 && parts[3] == "testing" {
			targetPath := filepath.Join(absRoot, "repos", "apps", parts[2], "testing", parts[4])
			if !DirExists(fs, targetPath) {
				return Target{}, fmt.Errorf("test path does not exist: %s", targetPath)
			}
			return Target{
				Kind: TargetTest,
				App:  parts[2],
				Name: parts[4],
				Path: targetPath,
			}, nil
		}
	case "libs":
		if len(parts) >= 3 {
			targetPath := filepath.Join(absRoot, "repos", "libs", parts[2])
			if !DirExists(fs, targetPath) {
				return Target{}, fmt.Errorf("library path does not exist: %s", targetPath)
			}
			return Target{
				Kind: TargetGlobalLib,
				Name: parts[2],
				Path: targetPath,
			}, nil
		}
	case "projects":
		if len(parts) >= 3 {
			targetPath := filepath.Join(absRoot, "repos", "projects", parts[2])
			if !DirExists(fs, targetPath) {
				return Target{}, fmt.Errorf("project path does not exist: %s", targetPath)
			}
			return Target{
				Kind: TargetProject,
				Name: parts[2],
				Path: targetPath,
			}, nil
		}
	}

	return Target{}, fmt.Errorf("current directory is not a supported install target")
}

// ValidateTargetRegistration ensures the target exists in the workspace config.
func ValidateTargetRegistration(target Target, config WorkspaceConfig) error {
	switch target.Kind {
	case TargetService:
		appIndex := FindAppIndex(config, target.App)
		if appIndex == -1 {
			return fmt.Errorf("app not registered in workspace: %s", target.App)
		}
		if FindServiceIndex(config.Apps[appIndex], target.Name) == -1 {
			return fmt.Errorf("service not registered in workspace: %s", target.Name)
		}
	case TargetAppLib:
		appIndex := FindAppIndex(config, target.App)
		if appIndex == -1 {
			return fmt.Errorf("app not registered in workspace: %s", target.App)
		}
		if FindAppLibraryIndex(config.Apps[appIndex], target.Name) == -1 {
			return fmt.Errorf("library not registered in workspace: %s", target.Name)
		}
	case TargetGlobalLib:
		libIndex := FindGlobalLibraryIndex(config, target.Name)
		if libIndex == -1 {
			return fmt.Errorf("library not registered in workspace: %s", target.Name)
		}
	case TargetProject:
		projectIndex := FindProjectIndex(config, target.Name)
		if projectIndex == -1 {
			return fmt.Errorf("project not registered in workspace: %s", target.Name)
		}
	case TargetTest:
		appIndex := FindAppIndex(config, target.App)
		if appIndex == -1 {
			return fmt.Errorf("app not registered in workspace: %s", target.App)
		}
		if FindAppTestIndex(config.Apps[appIndex], target.Name) == -1 {
			return fmt.Errorf("test not registered in workspace: %s", target.Name)
		}
	case TargetTemplate:
		if FindTemplateIndex(config, target.Name) == -1 {
			return fmt.Errorf("template not registered in workspace: %s", target.Name)
		}
	default:
		return fmt.Errorf("unsupported install target")
	}
	return nil
}

func ValidateWorkspaceConfig(config WorkspaceConfig) error {
	var problems []string
	if err := validatePorts(config.Ports); err != nil {
		problems = append(problems, err.Error())
	}
	for _, app := range config.Apps {
		if err := validateRepoVariableCollisions(app.Variables, reservedAppRepoFields); err != nil {
			problems = append(problems, fmt.Sprintf("app %s variables: %s", app.Name, err.Error()))
		}
		serviceNames := map[string]struct{}{}
		libNames := map[string]struct{}{}
		for _, service := range app.Services {
			if _, exists := serviceNames[service.Name]; exists && !isTemplateName(service.Name) {
				problems = append(problems, fmt.Sprintf("app %s service %s: duplicate", app.Name, service.Name))
			}
			serviceNames[service.Name] = struct{}{}
			if err := validateDeps(service.Deps); err != nil {
				problems = append(problems, fmt.Sprintf("app %s service %s deps: %s", app.Name, service.Name, err.Error()))
			}
			if err := validateRepoVariableCollisions(service.Variables, reservedServiceFields); err != nil {
				problems = append(problems, fmt.Sprintf("app %s service %s variables: %s", app.Name, service.Name, err.Error()))
			}
		}
		for _, lib := range app.Libraries {
			if _, exists := libNames[lib.Name]; exists && !isTemplateName(lib.Name) {
				problems = append(problems, fmt.Sprintf("app %s library %s: duplicate", app.Name, lib.Name))
			}
			libNames[lib.Name] = struct{}{}
			if err := validateDeps(lib.Deps); err != nil {
				problems = append(problems, fmt.Sprintf("app %s library %s deps: %s", app.Name, lib.Name, err.Error()))
			}
			if err := validateRepoVariableCollisions(lib.Variables, reservedAppLibFields); err != nil {
				problems = append(problems, fmt.Sprintf("app %s library %s variables: %s", app.Name, lib.Name, err.Error()))
			}
		}
		testNames := map[string]struct{}{}
		for _, test := range app.Testing {
			if _, exists := testNames[test.Name]; exists && !isTemplateName(test.Name) {
				problems = append(problems, fmt.Sprintf("app %s test %s: duplicate", app.Name, test.Name))
			}
			testNames[test.Name] = struct{}{}
			if err := validateDeps(test.Deps); err != nil {
				problems = append(problems, fmt.Sprintf("app %s test %s deps: %s", app.Name, test.Name, err.Error()))
			}
		}
	}
	globalLibs := map[string]struct{}{}
	for _, lib := range config.Libraries {
		if _, exists := globalLibs[lib.Name]; exists && !isTemplateName(lib.Name) {
			problems = append(problems, fmt.Sprintf("library %s: duplicate", lib.Name))
		}
		globalLibs[lib.Name] = struct{}{}
		if err := validateDeps(lib.Deps); err != nil {
			problems = append(problems, fmt.Sprintf("library %s deps: %s", lib.Name, err.Error()))
		}
		if err := validateRepoVariableCollisions(lib.Variables, reservedGlobalLibFields); err != nil {
			problems = append(problems, fmt.Sprintf("library %s variables: %s", lib.Name, err.Error()))
		}
	}
	projects := map[string]struct{}{}
	for _, project := range config.Projects {
		if _, exists := projects[project.Name]; exists && !isTemplateName(project.Name) {
			problems = append(problems, fmt.Sprintf("project %s: duplicate", project.Name))
		}
		projects[project.Name] = struct{}{}
		if err := validateDeps(project.Deps); err != nil {
			problems = append(problems, fmt.Sprintf("project %s deps: %s", project.Name, err.Error()))
		}
		if err := validateRepoVariableCollisions(project.Variables, reservedProjectFields); err != nil {
			problems = append(problems, fmt.Sprintf("project %s variables: %s", project.Name, err.Error()))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf(strings.Join(problems, "\n"))
	}
	return nil
}

// reservedProjectFields, reservedAppLibFields, reservedGlobalLibFields,
// reservedServiceFields, and reservedAppRepoFields enumerate the {{repo:*}}
// names that are auto-derived from each kind of repo entry. Users may not
// shadow these via a `variables` block.
var (
	reservedProjectFields   = []string{"name", "kind", "path", "scopes", "remote"}
	reservedAppRepoFields   = []string{"name", "kind", "path", "scopes", "remote"}
	reservedGlobalLibFields = []string{"name", "kind", "path", "scopes", "remote"}
	reservedAppLibFields    = []string{"name", "kind", "path", "scopes", "remote", "app"}
	reservedServiceFields   = []string{"name", "kind", "path", "scopes", "remote", "port", "image_name", "image_tag", "dockerfile", "container_file", "image_path", "app"}
)

func validateRepoVariableCollisions(user map[string]string, reserved []string) error {
	if len(user) == 0 {
		return nil
	}
	reservedSet := make(map[string]struct{}, len(reserved))
	for _, name := range reserved {
		reservedSet[name] = struct{}{}
	}
	for key := range user {
		if _, exists := reservedSet[key]; exists {
			return fmt.Errorf("%q collides with reserved repo field", key)
		}
	}
	return nil
}

func validatePorts(ports WorkspacePorts) error {
	if ports.Allowed.Min <= 0 {
		return fmt.Errorf("ports.allowed.min: required")
	}
	if ports.Allowed.Max <= 0 {
		return fmt.Errorf("ports.allowed.max: required")
	}
	if ports.Allowed.Max < ports.Allowed.Min {
		return fmt.Errorf("ports.allowed: max must be >= min")
	}
	seen := map[int]struct{}{}
	for _, entry := range ports.Reserved {
		if strings.TrimSpace(entry.Name) == "" {
			return fmt.Errorf("ports.reserved.name: required")
		}
		if entry.Port <= 0 {
			return fmt.Errorf("ports.reserved.port: required")
		}
		if _, exists := seen[entry.Port]; exists {
			return fmt.Errorf("ports.reserved.port: duplicate %d", entry.Port)
		}
		seen[entry.Port] = struct{}{}
	}
	return nil
}

func validateDeps(deps []WorkspaceDep) error {
	seen := map[string]struct{}{}
	for _, dep := range deps {
		lib := strings.TrimSpace(dep.Lib)
		if lib == "" {
			return fmt.Errorf("empty dependency")
		}
		from := strings.TrimSpace(dep.From)
		if from == "" {
			return fmt.Errorf("dependency %s missing source", lib)
		}
		key := fmt.Sprintf("%s|%s", lib, from)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate dependency %s from %s", lib, from)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func makeGlobalDeps(deps ...string) []WorkspaceDep {
	out := make([]WorkspaceDep, 0, len(deps))
	for _, dep := range deps {
		if strings.TrimSpace(dep) == "" {
			continue
		}
		out = append(out, WorkspaceDep{
			Lib:  dep,
			From: "global",
		})
	}
	return out
}

func ReadRepoConfig(fs afero.Fs, root string) (RepoConfig, error) {
	configPath := filepath.Join(root, "ocean.config.json")
	payload, err := afero.ReadFile(fs, configPath)
	if err != nil {
		return RepoConfig{}, err
	}
	var config RepoConfig
	if err := json.Unmarshal(payload, &config); err != nil {
		return RepoConfig{}, err
	}
	config.Language = strings.TrimSpace(config.Language)
	config.Type = strings.TrimSpace(config.Type)
	return config, nil
}

func WriteRepoConfig(fs afero.Fs, root string, config RepoConfig) error {
	configPath := filepath.Join(root, "ocean.config.json")
	payload, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	return afero.WriteFile(fs, configPath, payload, 0o644)
}

func RepoCommand(config RepoConfig, kind string) (string, error) {
	switch kind {
	case "build":
		if strings.TrimSpace(config.Tasks.Build) != "" {
			return config.Tasks.Build, nil
		}
		return config.Build, nil
	case "test":
		if strings.TrimSpace(config.Tasks.Test) != "" {
			return config.Tasks.Test, nil
		}
		return config.Test, nil
	case "install":
		if strings.TrimSpace(config.Tasks.Install) != "" {
			return config.Tasks.Install, nil
		}
		return config.Install, nil
	case "add":
		if strings.TrimSpace(config.Tasks.Add) != "" {
			return config.Tasks.Add, nil
		}
		return config.Add, nil
	case "uninstall":
		if strings.TrimSpace(config.Tasks.Uninstall) != "" {
			return config.Tasks.Uninstall, nil
		}
		return config.Uninstall, nil
	case "contain":
		return config.Tasks.Contain, nil
	case "publish":
		return config.Tasks.Publish, nil
	case "run":
		return config.Tasks.Run, nil
	case "setup":
		return config.Tasks.Setup, nil
	case "prebuild":
		return config.Tasks.Prebuild, nil
	default:
		return "", fmt.Errorf("unsupported command kind: %s", kind)
	}
}

func ReadRepoCommand(fs afero.Fs, root string, kind string) (string, error) {
	config, err := ReadRepoConfig(fs, root)
	if err != nil {
		return "", err
	}
	return RepoCommand(config, kind)
}

func isTemplateName(value string) bool {
	return strings.Contains(value, "{{") && strings.Contains(value, "}}")
}

func RunBuild(cmd *cobra.Command, label string, targetPath string, hashPath string, root string) error {
	buildCmd, err := readBuildCommand(targetPath)
	if err != nil {
		return err
	}
	if strings.TrimSpace(buildCmd) == "" {
		fmt.Fprintf(cmd.OutOrStdout(), "build skipped for %s: no build command\n", label)
		return nil
	}

	newHash, err := CalcRepoHash(afero.NewOsFs(), root, targetPath)
	if err != nil {
		return err
	}
	prevHash, hasPrevHash, err := ReadHashFile(afero.NewOsFs(), hashPath)
	if err != nil {
		return err
	}
	if hasPrevHash && prevHash == newHash {
		fmt.Fprintf(cmd.OutOrStdout(), "build skipped for %s: no changes\n", label)
		return nil
	}

	prebuildCmd, _ := ReadRepoCommand(afero.NewOsFs(), targetPath, "prebuild")
	if strings.TrimSpace(prebuildCmd) != "" {
		preCmd := exec.Command("bash", "-lc", prebuildCmd)
		preCmd.Dir = targetPath
		preCmd.Stdout = cmd.OutOrStdout()
		preCmd.Stderr = cmd.ErrOrStderr()
		if err := preCmd.Run(); err != nil {
			return err
		}
	}

	execCmd := exec.Command("bash", "-lc", buildCmd)
	execCmd.Dir = targetPath
	execCmd.Stdout = cmd.OutOrStdout()
	execCmd.Stderr = cmd.ErrOrStderr()
	if err := execCmd.Run(); err != nil {
		return err
	}

	postHash, err := CalcRepoHash(afero.NewOsFs(), root, targetPath)
	if err != nil {
		return err
	}
	return WriteHashFile(afero.NewOsFs(), hashPath, postHash)
}

func RunCheck(cmd *cobra.Command, label string, targetPath string, hashPath string, passThrough []string, root string) error {
	testCmd, err := readTestCommand(targetPath)
	if err != nil {
		return err
	}
	if strings.TrimSpace(testCmd) == "" {
		fmt.Fprintf(cmd.OutOrStdout(), "check skipped for %s: no test command\n", label)
		return nil
	}

	newHash, err := CalcRepoHash(afero.NewOsFs(), root, targetPath)
	if err != nil {
		return err
	}
	prevHash, hasPrevHash, err := ReadHashFile(afero.NewOsFs(), hashPath)
	if err != nil {
		return err
	}
	if hasPrevHash && prevHash == newHash {
		fmt.Fprintf(cmd.OutOrStdout(), "check skipped for %s: no changes\n", label)
		return nil
	}

	buildCmd, err := readBuildCommand(targetPath)
	if err != nil {
		return err
	}
	if strings.TrimSpace(buildCmd) != "" {
		buildHashPath := MakeBuildHashPath(hashPath)
		prevBuildHash, hasPrevBuildHash, err := ReadHashFile(afero.NewOsFs(), buildHashPath)
		if err != nil {
			return err
		}
		if !hasPrevBuildHash || prevBuildHash != newHash {
			execCmd := exec.Command("bash", "-lc", buildCmd)
			execCmd.Dir = targetPath
			execCmd.Stdout = cmd.OutOrStdout()
			execCmd.Stderr = cmd.ErrOrStderr()
			if err := execCmd.Run(); err != nil {
				return err
			}
			if err := WriteHashFile(afero.NewOsFs(), buildHashPath, newHash); err != nil {
				return err
			}
		}
	}

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	execCmd := exec.Command("bash", "-lc", buildTestCommand(testCmd, passThrough))
	execCmd.Dir = targetPath
	execCmd.Stdout = io.MultiWriter(cmd.OutOrStdout(), &stdoutBuf)
	execCmd.Stderr = io.MultiWriter(cmd.ErrOrStderr(), &stderrBuf)
	startedAt := time.Now()
	execErr := execCmd.Run()

	if writeErr := WriteCheckResults(afero.NewOsFs(), root, targetPath, label, startedAt, stdoutBuf.Bytes(), stderrBuf.Bytes(), execErr); writeErr != nil {
		if execErr != nil {
			return fmt.Errorf("check failed: %w; results write failed: %v", execErr, writeErr)
		}
		return writeErr
	}
	if execErr != nil {
		return execErr
	}

	return WriteHashFile(afero.NewOsFs(), hashPath, newHash)
}

func readBuildCommand(targetPath string) (string, error) {
	return ReadRepoCommand(afero.NewOsFs(), targetPath, "build")
}

func readTestCommand(targetPath string) (string, error) {
	return ReadRepoCommand(afero.NewOsFs(), targetPath, "test")
}

func buildTestCommand(base string, passThrough []string) string {
	if len(passThrough) == 0 {
		return base
	}
	parts := make([]string, 0, len(passThrough))
	for _, arg := range passThrough {
		parts = append(parts, shellQuote(arg))
	}
	return base + " " + strings.Join(parts, " ")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	needsQuotes := strings.IndexFunc(value, func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', '\r', '"', '\'', '\\', '$', '`':
			return true
		default:
			return false
		}
	}) != -1
	if !needsQuotes {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// FindTargetScopes returns the scope list for a resolved Target from workspace config.
func FindTargetScopes(config WorkspaceConfig, target Target) []string {
	switch target.Kind {
	case TargetGlobalLib:
		idx := FindGlobalLibraryIndex(config, target.Name)
		if idx == -1 {
			return nil
		}
		return config.Libraries[idx].Scopes
	case TargetProject:
		idx := FindProjectIndex(config, target.Name)
		if idx == -1 {
			return nil
		}
		return config.Projects[idx].Scopes
	case TargetService:
		appIdx := FindAppIndex(config, target.App)
		if appIdx == -1 {
			return nil
		}
		svcIdx := FindServiceIndex(config.Apps[appIdx], target.Name)
		if svcIdx == -1 {
			return nil
		}
		return config.Apps[appIdx].Services[svcIdx].Scopes
	case TargetAppLib:
		appIdx := FindAppIndex(config, target.App)
		if appIdx == -1 {
			return nil
		}
		libIdx := FindAppLibraryIndex(config.Apps[appIdx], target.Name)
		if libIdx == -1 {
			return nil
		}
		return config.Apps[appIdx].Libraries[libIdx].Scopes
	case TargetTest:
		appIdx := FindAppIndex(config, target.App)
		if appIdx == -1 {
			return nil
		}
		testIdx := FindAppTestIndex(config.Apps[appIdx], target.Name)
		if testIdx == -1 {
			return nil
		}
		return config.Apps[appIdx].Testing[testIdx].Scopes
	}
	return nil
}

// HasCommonScope returns true if slices a and b share at least one element.
func HasCommonScope(a, b []string) bool {
	set := make(map[string]struct{}, len(a))
	for _, s := range a {
		set[s] = struct{}{}
	}
	for _, s := range b {
		if _, ok := set[s]; ok {
			return true
		}
	}
	return false
}

// AddScopeToWorkspaceTarget appends a scope to a target's entry in the workspace config.
// It is a no-op if the scope is already present.
func AddScopeToWorkspaceTarget(config WorkspaceConfig, target Target, scope string) (WorkspaceConfig, error) {
	switch target.Kind {
	case TargetGlobalLib:
		idx := FindGlobalLibraryIndex(config, target.Name)
		if idx == -1 {
			return config, fmt.Errorf("library not registered in workspace: %s", target.Name)
		}
		if !containsString(config.Libraries[idx].Scopes, scope) {
			config.Libraries[idx].Scopes = append(config.Libraries[idx].Scopes, scope)
		}
	case TargetProject:
		idx := FindProjectIndex(config, target.Name)
		if idx == -1 {
			return config, fmt.Errorf("project not registered in workspace: %s", target.Name)
		}
		if !containsString(config.Projects[idx].Scopes, scope) {
			config.Projects[idx].Scopes = append(config.Projects[idx].Scopes, scope)
		}
	case TargetService:
		appIdx := FindAppIndex(config, target.App)
		if appIdx == -1 {
			return config, fmt.Errorf("app not registered in workspace: %s", target.App)
		}
		svcIdx := FindServiceIndex(config.Apps[appIdx], target.Name)
		if svcIdx == -1 {
			return config, fmt.Errorf("service not registered in workspace: %s", target.Name)
		}
		if !containsString(config.Apps[appIdx].Services[svcIdx].Scopes, scope) {
			config.Apps[appIdx].Services[svcIdx].Scopes = append(config.Apps[appIdx].Services[svcIdx].Scopes, scope)
		}
	case TargetAppLib:
		appIdx := FindAppIndex(config, target.App)
		if appIdx == -1 {
			return config, fmt.Errorf("app not registered in workspace: %s", target.App)
		}
		libIdx := FindAppLibraryIndex(config.Apps[appIdx], target.Name)
		if libIdx == -1 {
			return config, fmt.Errorf("library not registered in workspace: %s", target.Name)
		}
		if !containsString(config.Apps[appIdx].Libraries[libIdx].Scopes, scope) {
			config.Apps[appIdx].Libraries[libIdx].Scopes = append(config.Apps[appIdx].Libraries[libIdx].Scopes, scope)
		}
	case TargetTest:
		appIdx := FindAppIndex(config, target.App)
		if appIdx == -1 {
			return config, fmt.Errorf("app not registered in workspace: %s", target.App)
		}
		testIdx := FindAppTestIndex(config.Apps[appIdx], target.Name)
		if testIdx == -1 {
			return config, fmt.Errorf("test not registered in workspace: %s", target.Name)
		}
		if !containsString(config.Apps[appIdx].Testing[testIdx].Scopes, scope) {
			config.Apps[appIdx].Testing[testIdx].Scopes = append(config.Apps[appIdx].Testing[testIdx].Scopes, scope)
		}
	default:
		return config, fmt.Errorf("unsupported target kind")
	}
	return config, nil
}

// RemoveScopeFromWorkspaceTarget removes a scope from a target's entry in the workspace config.
// It is a no-op if the scope is not present.
func RemoveScopeFromWorkspaceTarget(config WorkspaceConfig, target Target, scope string) (WorkspaceConfig, error) {
	switch target.Kind {
	case TargetGlobalLib:
		idx := FindGlobalLibraryIndex(config, target.Name)
		if idx == -1 {
			return config, fmt.Errorf("library not registered in workspace: %s", target.Name)
		}
		config.Libraries[idx].Scopes = removeString(config.Libraries[idx].Scopes, scope)
	case TargetProject:
		idx := FindProjectIndex(config, target.Name)
		if idx == -1 {
			return config, fmt.Errorf("project not registered in workspace: %s", target.Name)
		}
		config.Projects[idx].Scopes = removeString(config.Projects[idx].Scopes, scope)
	case TargetService:
		appIdx := FindAppIndex(config, target.App)
		if appIdx == -1 {
			return config, fmt.Errorf("app not registered in workspace: %s", target.App)
		}
		svcIdx := FindServiceIndex(config.Apps[appIdx], target.Name)
		if svcIdx == -1 {
			return config, fmt.Errorf("service not registered in workspace: %s", target.Name)
		}
		config.Apps[appIdx].Services[svcIdx].Scopes = removeString(config.Apps[appIdx].Services[svcIdx].Scopes, scope)
	case TargetAppLib:
		appIdx := FindAppIndex(config, target.App)
		if appIdx == -1 {
			return config, fmt.Errorf("app not registered in workspace: %s", target.App)
		}
		libIdx := FindAppLibraryIndex(config.Apps[appIdx], target.Name)
		if libIdx == -1 {
			return config, fmt.Errorf("library not registered in workspace: %s", target.Name)
		}
		config.Apps[appIdx].Libraries[libIdx].Scopes = removeString(config.Apps[appIdx].Libraries[libIdx].Scopes, scope)
	case TargetTest:
		appIdx := FindAppIndex(config, target.App)
		if appIdx == -1 {
			return config, fmt.Errorf("app not registered in workspace: %s", target.App)
		}
		testIdx := FindAppTestIndex(config.Apps[appIdx], target.Name)
		if testIdx == -1 {
			return config, fmt.Errorf("test not registered in workspace: %s", target.Name)
		}
		config.Apps[appIdx].Testing[testIdx].Scopes = removeString(config.Apps[appIdx].Testing[testIdx].Scopes, scope)
	default:
		return config, fmt.Errorf("unsupported target kind")
	}
	return config, nil
}

// FindDepsReliantOnScope returns human-readable descriptions of cross-app dependency
// relationships that will be broken if scope is removed from target (REQ 7.3).
func FindDepsReliantOnScope(config WorkspaceConfig, target Target, scope string) []string {
	// Only app-libs can be cross-app payload sources; check if target is an app-lib.
	if target.Kind != TargetAppLib {
		return nil
	}

	// Build the scopes target would have after removal.
	currentScopes := FindTargetScopes(config, target)
	remainingScopes := removeString(currentScopes, scope)

	var warnings []string
	for _, app := range config.Apps {
		for _, svc := range app.Services {
			if app.Name == target.App {
				continue // same app, not cross-app
			}
			for _, dep := range svc.Deps {
				if dep.Lib != target.Name {
					continue
				}
				// This service in a different app depends on the target lib.
				// Check if it currently relies on the scope being removed.
				dependentScopes := svc.Scopes
				if HasCommonScope(currentScopes, dependentScopes) && !HasCommonScope(remainingScopes, dependentScopes) {
					warnings = append(warnings, fmt.Sprintf(
						"warning: removing scope %q from %s/%s may break cross-app dependency from service %s/%s",
						scope, target.App, target.Name, app.Name, svc.Name,
					))
				}
			}
		}
		for _, lib := range app.Libraries {
			if app.Name == target.App {
				continue
			}
			for _, dep := range lib.Deps {
				if dep.Lib != target.Name {
					continue
				}
				dependentScopes := lib.Scopes
				if HasCommonScope(currentScopes, dependentScopes) && !HasCommonScope(remainingScopes, dependentScopes) {
					warnings = append(warnings, fmt.Sprintf(
						"warning: removing scope %q from %s/%s may break cross-app dependency from library %s/%s",
						scope, target.App, target.Name, app.Name, lib.Name,
					))
				}
			}
		}
		for _, test := range app.Testing {
			if app.Name == target.App {
				continue
			}
			for _, dep := range test.Deps {
				if dep.Lib != target.Name {
					continue
				}
				dependentScopes := test.Scopes
				if HasCommonScope(currentScopes, dependentScopes) && !HasCommonScope(remainingScopes, dependentScopes) {
					warnings = append(warnings, fmt.Sprintf(
						"warning: removing scope %q from %s/%s may break cross-app dependency from test %s/%s",
						scope, target.App, target.Name, app.Name, test.Name,
					))
				}
			}
		}
	}
	return warnings
}

// AddScope adds a scope name to a target in both workspace config and ocean.config.json (REQ 7.1).
func AddScope(cmd *cobra.Command, fs afero.Fs, scopeName string, targetName string) error {
	root, err := EnsureWorkspaceRoot(fs)
	if err != nil {
		return err
	}
	config, err := ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	target, err := ResolveTargetByName(config, root, targetName)
	if err != nil {
		return err
	}
	updatedConfig, err := AddScopeToWorkspaceTarget(config, target, scopeName)
	if err != nil {
		return err
	}
	if err := WriteWorkspaceConfig(fs, updatedConfig); err != nil {
		return err
	}
	repoConfig, err := ReadRepoConfig(fs, target.Path)
	if err != nil {
		repoConfig = RepoConfig{}
	}
	if !containsString(repoConfig.Scopes, scopeName) {
		repoConfig.Scopes = append(repoConfig.Scopes, scopeName)
	}
	return WriteRepoConfig(fs, target.Path, repoConfig)
}

// RemoveScope removes a scope name from a target in both workspace config and ocean.config.json (REQ 7.2/7.3).
func RemoveScope(cmd *cobra.Command, fs afero.Fs, scopeName string, targetName string) error {
	root, err := EnsureWorkspaceRoot(fs)
	if err != nil {
		return err
	}
	config, err := ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	target, err := ResolveTargetByName(config, root, targetName)
	if err != nil {
		return err
	}
	for _, warning := range FindDepsReliantOnScope(config, target, scopeName) {
		fmt.Fprintln(cmd.OutOrStdout(), warning)
	}
	updatedConfig, err := RemoveScopeFromWorkspaceTarget(config, target, scopeName)
	if err != nil {
		return err
	}
	if err := WriteWorkspaceConfig(fs, updatedConfig); err != nil {
		return err
	}
	repoConfig, err := ReadRepoConfig(fs, target.Path)
	if err != nil {
		return err
	}
	repoConfig.Scopes = removeString(repoConfig.Scopes, scopeName)
	return WriteRepoConfig(fs, target.Path, repoConfig)
}

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) []string {
	out := make([]string, 0, len(slice))
	for _, v := range slice {
		if v != s {
			out = append(out, v)
		}
	}
	return out
}
