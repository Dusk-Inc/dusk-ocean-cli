package workspace

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/compose"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/hash"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/j_unit"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/tree"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

type WorkspaceConfig struct {
	Workspace      string             `json:"workspace"`
	Version        string             `json:"version,omitempty"`
	DockerRegistry string             `json:"docker_registry"`
	Ports          WorkspacePorts     `json:"ports"`
	Apps           []WorkspaceApp     `json:"apps"`
	Libraries      []WorkspaceLibrary `json:"libraries"`
	Projects       []WorkspaceProject `json:"projects"`
}

type WorkspacePorts struct {
	Allowed  WorkspacePortRange      `json:"allowed"`
	Reserved []WorkspaceReservedPort `json:"reserved"`
}

type WorkspacePortRange struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

type WorkspaceReservedPort struct {
	Name string `json:"name"`
	Port int    `json:"port"`
}

type WorkspaceApp struct {
	Name      string             `json:"name"`
	Services  []WorkspaceService `json:"services"`
	Libraries []WorkspaceLibrary `json:"libraries"`
}

type WorkspaceImage struct {
	Name string `json:"name"`
	Tag  string `json:"tag"`
}

type WorkspaceService struct {
	Name       string         `json:"name"`
	Port       string         `json:"port"`
	Image      WorkspaceImage `json:"image"`
	Dockerfile string         `json:"Dockerfile"`
	Deps       []string       `json:"deps"`
}

type WorkspaceLibrary struct {
	Name string   `json:"name"`
	Deps []string `json:"deps"`
}

type WorkspaceProject struct {
	Name string   `json:"name"`
	Deps []string `json:"deps"`
}

const defaultImageTag = "dev"

type InitOptions struct {
	Name     string
	Registry string
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
		Deps: deps,
	}
}

func MakeProject(name string, deps ...string) WorkspaceProject {
	return WorkspaceProject{
		Name: name,
		Deps: deps,
	}
}

func MakeApp(name string, libraries []WorkspaceLibrary) WorkspaceApp {
	return WorkspaceApp{
		Name:      name,
		Services:  nil,
		Libraries: libraries,
	}
}

func InitWorkspace(fs afero.Fs, out io.Writer, opts InitOptions) error {
	if opts.Name == "" {
		return fmt.Errorf("--name is required")
	}
	if opts.Registry == "" {
		return fmt.Errorf("--registry is required")
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
		filepath.Join("repos", "templates"),
		filepath.Join("repos", "libs"),
		filepath.Join("repos", "containers"),
		filepath.Join("repos", "templates", "apps"),
		filepath.Join("repos", "templates", "apps", "services"),
		filepath.Join("repos", "templates", "apps", "libs"),
		filepath.Join("repos", "templates", "apps", "jobs"),
		filepath.Join("repos", "templates", "apps", "testing"),
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

	appTemplatePath := filepath.Join("repos", "templates", "apps")
	if err := ensureFile(fs, out, filepath.Join(appTemplatePath, "docker-compose.yml"), ""); err != nil {
		return err
	}
	if err := ensureFile(fs, out, filepath.Join(appTemplatePath, "docker-compose.dev.yml"), ""); err != nil {
		return err
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

	workspaceConfig := WorkspaceConfig{
		Workspace:      opts.Name,
		Version:        "",
		DockerRegistry: opts.Registry,
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
			if config.Apps[i].Services[j].Deps == nil {
				config.Apps[i].Services[j].Deps = []string{}
			}
			if config.Apps[i].Services[j].Image.Name == "" {
				config.Apps[i].Services[j].Image.Name = fmt.Sprintf("%s__%s", config.Apps[i].Name, config.Apps[i].Services[j].Name)
			}
			if config.Apps[i].Services[j].Image.Tag == "" {
				config.Apps[i].Services[j].Image.Tag = defaultImageTag
			}
		}
		for j := range config.Apps[i].Libraries {
			if config.Apps[i].Libraries[j].Deps == nil {
				config.Apps[i].Libraries[j].Deps = []string{}
			}
		}
	}
	for i := range config.Libraries {
		if config.Libraries[i].Deps == nil {
			config.Libraries[i].Deps = []string{}
		}
	}
	for i := range config.Projects {
		if config.Projects[i].Deps == nil {
			config.Projects[i].Deps = []string{}
		}
	}
	return config
}

func ValidateWorkspaceConfig(config WorkspaceConfig) error {
	var problems []string
	if err := validatePorts(config.Ports); err != nil {
		problems = append(problems, err.Error())
	}
	for _, app := range config.Apps {
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
		}
		for _, lib := range app.Libraries {
			if _, exists := libNames[lib.Name]; exists && !isTemplateName(lib.Name) {
				problems = append(problems, fmt.Sprintf("app %s library %s: duplicate", app.Name, lib.Name))
			}
			libNames[lib.Name] = struct{}{}
			if err := validateDeps(lib.Deps); err != nil {
				problems = append(problems, fmt.Sprintf("app %s library %s deps: %s", app.Name, lib.Name, err.Error()))
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
	}
	if len(problems) > 0 {
		return fmt.Errorf(strings.Join(problems, "\n"))
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

func validateDeps(deps []string) error {
	seen := map[string]struct{}{}
	for _, dep := range deps {
		trimmed := strings.TrimSpace(dep)
		if trimmed == "" {
			return fmt.Errorf("empty dependency")
		}
		if _, exists := seen[trimmed]; exists {
			return fmt.Errorf("duplicate dependency %s", trimmed)
		}
		seen[trimmed] = struct{}{}
	}
	return nil
}

func isTemplateName(value string) bool {
	return strings.Contains(value, "{{") && strings.Contains(value, "}}")
}

func RunBuild(cmd *cobra.Command, label string, targetPath string, hashPath string) error {
	buildCmd, err := readBuildCommand(targetPath)
	if err != nil {
		return err
	}
	if strings.TrimSpace(buildCmd) == "" {
		fmt.Fprintf(cmd.OutOrStdout(), "build skipped for %s: no build command\n", label)
		return nil
	}

	root, err := tree.GetRoot()
	if err != nil {
		return err
	}
	ignorePatterns, err := ReadGitignorePatterns(afero.NewOsFs(), root)
	if err != nil {
		return err
	}
	newHash, err := hash.CalcDirHash(afero.NewOsFs(), targetPath, ignorePatterns)
	if err != nil {
		return err
	}
	prevHash, hasPrevHash, err := hash.ReadHashFile(afero.NewOsFs(), hashPath)
	if err != nil {
		return err
	}
	if hasPrevHash && prevHash == newHash {
		fmt.Fprintf(cmd.OutOrStdout(), "build skipped for %s: no changes\n", label)
		return nil
	}

	execCmd := exec.Command("bash", "-lc", buildCmd)
	execCmd.Dir = targetPath
	execCmd.Stdout = cmd.OutOrStdout()
	execCmd.Stderr = cmd.ErrOrStderr()
	if err := execCmd.Run(); err != nil {
		return err
	}

	return hash.WriteHashFile(afero.NewOsFs(), hashPath, newHash)
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

	ignorePatterns, err := ReadGitignorePatterns(afero.NewOsFs(), root)
	if err != nil {
		return err
	}
	newHash, err := hash.CalcDirHash(afero.NewOsFs(), targetPath, ignorePatterns)
	if err != nil {
		return err
	}
	prevHash, hasPrevHash, err := hash.ReadHashFile(afero.NewOsFs(), hashPath)
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
		buildHashPath := hash.MakeBuildHashPath(hashPath)
		prevBuildHash, hasPrevBuildHash, err := hash.ReadHashFile(afero.NewOsFs(), buildHashPath)
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
			if err := hash.WriteHashFile(afero.NewOsFs(), buildHashPath, newHash); err != nil {
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

	if writeErr := j_unit.WriteCheckResults(afero.NewOsFs(), root, targetPath, label, startedAt, stdoutBuf.Bytes(), stderrBuf.Bytes(), execErr); writeErr != nil {
		if execErr != nil {
			return fmt.Errorf("check failed: %w; results write failed: %v", execErr, writeErr)
		}
		return writeErr
	}
	if execErr != nil {
		return execErr
	}

	return hash.WriteHashFile(afero.NewOsFs(), hashPath, newHash)
}

func readBuildCommand(targetPath string) (string, error) {
	config, err := ReadRepoConfig(afero.NewOsFs(), targetPath)
	if err != nil {
		return "", err
	}
	return RepoCommand(config, "build")
}

func readTestCommand(targetPath string) (string, error) {
	config, err := ReadRepoConfig(afero.NewOsFs(), targetPath)
	if err != nil {
		return "", err
	}
	return RepoCommand(config, "test")
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

type ComposeSnapshot struct {
	Images map[string]string
	Ports  map[string][]string
}

func ValidateComposeConsistency(fs afero.Fs, root string) error {
	apps, err := tree.GetApps()
	if err != nil {
		return err
	}
	var problems []string
	for _, app := range apps {
		appPath := filepath.Join(root, "repos", "apps", app.Name)
		paths := []string{
			filepath.Join(appPath, "docker-compose.yml"),
			filepath.Join(appPath, "docker-compose.dev.yml"),
			filepath.Join(appPath, "docker-compose.hashi.yml"),
		}
		snapshots := map[string]ComposeSnapshot{}
		for _, path := range paths {
			if _, err := fs.Stat(path); err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return err
			}
			services, err := compose.ReadComposeServices(fs, path)
			if err != nil {
				return err
			}
			snapshots[path] = SnapshotServices(services)
			dupPorts := FindDuplicatePorts(services)
			if len(dupPorts) > 0 {
				for _, entry := range dupPorts {
					problems = append(problems, fmt.Sprintf("%s has duplicate port %s", path, entry))
				}
			}
		}
		images, ports := MergeSnapshots(snapshots)
		for name, image := range images {
			if image == "" {
				continue
			}
			for path, snapshot := range snapshots {
				if value, ok := snapshot.Images[name]; ok && value != "" && value != image {
					problems = append(problems, fmt.Sprintf("%s has mismatched image for %s", path, name))
				}
			}
		}
		for name, portSet := range ports {
			if len(portSet) == 0 {
				continue
			}
			for path, snapshot := range snapshots {
				if value, ok := snapshot.Ports[name]; ok && len(value) > 0 && !PortsEqual(value, portSet) {
					problems = append(problems, fmt.Sprintf("%s has mismatched ports for %s", path, name))
				}
			}
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf(strings.Join(problems, "\n"))
	}
	return nil
}

func SnapshotServices(services map[string]map[string]any) ComposeSnapshot {
	images := map[string]string{}
	ports := map[string][]string{}
	for name, service := range services {
		images[name] = compose.ReadServiceImage(service)
		ports[name] = NormalizePorts(compose.ReadServicePorts(service))
	}
	return ComposeSnapshot{
		Images: images,
		Ports:  ports,
	}
}

func NormalizePorts(ports []string) []string {
	out := make([]string, 0, len(ports))
	for _, port := range ports {
		value := strings.TrimSpace(port)
		if value != "" {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func MergeSnapshots(snapshots map[string]ComposeSnapshot) (map[string]string, map[string][]string) {
	images := map[string]string{}
	ports := map[string][]string{}
	for _, snapshot := range snapshots {
		for name, image := range snapshot.Images {
			if image == "" {
				continue
			}
			if existing, ok := images[name]; !ok || existing == "" {
				images[name] = image
			}
		}
		for name, portSet := range snapshot.Ports {
			if len(portSet) == 0 {
				continue
			}
			if _, ok := ports[name]; !ok {
				ports[name] = portSet
			}
		}
	}
	return images, ports
}

func PortsEqual(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func FindDuplicatePorts(services map[string]map[string]any) []string {
	seen := map[string]string{}
	var dupes []string
	for name, service := range services {
		for _, port := range NormalizePorts(compose.ReadServicePorts(service)) {
			host := compose.ReadHostPort(port)
			if host == "" {
				continue
			}
			if existing, ok := seen[host]; ok && existing != name {
				dupes = append(dupes, host)
			} else {
				seen[host] = name
			}
		}
	}
	sort.Strings(dupes)
	return dupes
}
