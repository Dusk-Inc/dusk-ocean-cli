package functions

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
	"github.com/spf13/afero"
)

// VariableContext holds the four namespaces consulted by Substitute.
// Each map is keyed by the bare variable name (no namespace prefix).
type VariableContext struct {
	Env   map[string]string
	Var   map[string]string
	Ocean map[string]string
	Repo  map[string]string
}

// variablePattern matches {{ns:name}} tokens. The namespace and name are
// captured in groups 1 and 2 respectively. Whitespace inside the braces
// is tolerated.
var variablePattern = regexp.MustCompile(`\{\{\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*:\s*([^{}\s]+)\s*\}\}`)

// Substitute replaces every {{ns:name}} token in template with the matching
// value from ctx. Unknown namespaces and missing keys are hard errors —
// substitution never silently inserts an empty string.
func Substitute(template string, ctx VariableContext) (string, error) {
	var firstErr error
	out := variablePattern.ReplaceAllStringFunc(template, func(match string) string {
		if firstErr != nil {
			return match
		}
		parts := variablePattern.FindStringSubmatch(match)
		ns := parts[1]
		name := parts[2]

		var lookup map[string]string
		switch ns {
		case tokens.VarNsEnv:
			lookup = ctx.Env
		case tokens.VarNsVar:
			lookup = ctx.Var
		case tokens.VarNsOcean:
			lookup = ctx.Ocean
		case tokens.VarNsRepo:
			lookup = ctx.Repo
		default:
			firstErr = fmt.Errorf("unknown variable namespace %q in token %s", ns, match)
			return match
		}

		value, ok := lookup[name]
		if !ok {
			firstErr = fmt.Errorf("missing variable %s:%s referenced by template", ns, name)
			return match
		}
		return value
	})
	if firstErr != nil {
		return "", firstErr
	}
	return out, nil
}

// LoadEnvFile reads a workspace-root .env file and returns its KEY=VALUE pairs.
// A missing file is logged to out and returns an empty map (no error). This
// matches the project's "no silent defaults" guidance for developer-managed
// config files in docs/LEARNING.md.
func LoadEnvFile(fs afero.Fs, root string, out io.Writer) (map[string]string, error) {
	path := filepath.Join(root, ".env")
	f, err := fs.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(out, ".env not found at workspace root; proceeding with empty env namespace\n")
			return map[string]string{}, nil
		}
		return nil, err
	}
	defer f.Close()

	values := map[string]string{}
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			return nil, fmt.Errorf(".env line %d: expected KEY=VALUE", lineNum)
		}
		key := strings.TrimSpace(line[:eq])
		value := strings.TrimSpace(line[eq+1:])
		if key == "" {
			return nil, fmt.Errorf(".env line %d: empty key", lineNum)
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

// LoadWorkspaceVariables returns the user-defined workspace-level variables
// map. It is a thin convenience wrapper so callers do not need to remember
// that nil maps are valid.
func LoadWorkspaceVariables(config WorkspaceConfig) map[string]string {
	if config.Variables == nil {
		return map[string]string{}
	}
	return config.Variables
}

// BuildRepoVariables walks the workspace config to find the entry identified
// by kind/appName/repoName and returns its full {{repo:*}} map. Reserved
// fields (name, kind, path, scopes, remote, plus service/app extras) are
// auto-populated from the entry. User-declared variables on the entry are
// merged in afterward, but a user variable that collides with a reserved
// name is rejected with an error.
func BuildRepoVariables(config WorkspaceConfig, kind string, appName, repoName string) (map[string]string, error) {
	switch kind {
	case tokens.RepoKindProject:
		idx := FindProjectIndex(config, repoName)
		if idx == -1 {
			return nil, fmt.Errorf("project not registered in workspace: %s", repoName)
		}
		project := config.Projects[idx]
		reserved := map[string]string{
			"name":   project.Name,
			"kind":   tokens.RepoKindProject,
			"path":   filepath.Join("repos", "projects", project.Name),
			"scopes": strings.Join(project.Scopes, ","),
			"remote": project.Remote,
		}
		return mergeRepoVariables(reserved, project.Variables, kind, project.Name)

	case tokens.RepoKindLibrary:
		if appName != "" {
			appIdx := FindAppIndex(config, appName)
			if appIdx == -1 {
				return nil, fmt.Errorf("app not registered in workspace: %s", appName)
			}
			libIdx := FindAppLibraryIndex(config.Apps[appIdx], repoName)
			if libIdx == -1 {
				return nil, fmt.Errorf("library not registered in workspace: %s (app %s)", repoName, appName)
			}
			lib := config.Apps[appIdx].Libraries[libIdx]
			reserved := map[string]string{
				"name":   lib.Name,
				"kind":   tokens.RepoKindLibrary,
				"path":   filepath.Join("repos", "apps", appName, "libs", lib.Name),
				"scopes": strings.Join(lib.Scopes, ","),
				"remote": lib.Remote,
				"app":    appName,
			}
			return mergeRepoVariables(reserved, lib.Variables, kind, lib.Name)
		}
		idx := FindGlobalLibraryIndex(config, repoName)
		if idx == -1 {
			return nil, fmt.Errorf("library not registered in workspace: %s", repoName)
		}
		lib := config.Libraries[idx]
		reserved := map[string]string{
			"name":   lib.Name,
			"kind":   tokens.RepoKindLibrary,
			"path":   filepath.Join("repos", "libs", lib.Name),
			"scopes": strings.Join(lib.Scopes, ","),
			"remote": lib.Remote,
		}
		return mergeRepoVariables(reserved, lib.Variables, kind, lib.Name)

	case tokens.RepoKindApp:
		idx := FindAppIndex(config, repoName)
		if idx == -1 {
			return nil, fmt.Errorf("app not registered in workspace: %s", repoName)
		}
		app := config.Apps[idx]
		reserved := map[string]string{
			"name":   app.Name,
			"kind":   tokens.RepoKindApp,
			"path":   filepath.Join("repos", "apps", app.Name),
			"scopes": "",
			"remote": app.Remote,
		}
		return mergeRepoVariables(reserved, app.Variables, kind, app.Name)

	case tokens.RepoKindService:
		if appName == "" {
			return nil, fmt.Errorf("service variable lookup requires an app name")
		}
		appIdx := FindAppIndex(config, appName)
		if appIdx == -1 {
			return nil, fmt.Errorf("app not registered in workspace: %s", appName)
		}
		svcIdx := FindServiceIndex(config.Apps[appIdx], repoName)
		if svcIdx == -1 {
			return nil, fmt.Errorf("service not registered in workspace: %s (app %s)", repoName, appName)
		}
		svc := config.Apps[appIdx].Services[svcIdx]
		reserved := map[string]string{
			"name":           svc.Name,
			"kind":           tokens.RepoKindService,
			"path":           filepath.Join("repos", "apps", appName, "services", svc.Name),
			"scopes":         strings.Join(svc.Scopes, ","),
			"remote":         svc.Remote,
			"port":           svc.Port,
			"image_name":     svc.Image.Name,
			"image_tag":      svc.Image.Tag,
			"dockerfile":     svc.Dockerfile,
			"container_file": svc.ContainerFile,
			"image_path":     svc.ImagePath,
			"app":            appName,
		}
		return mergeRepoVariables(reserved, svc.Variables, kind, svc.Name)
	}
	return nil, fmt.Errorf("unknown repo kind: %s", kind)
}

// mergeRepoVariables overlays user-declared variables onto the reserved set.
// A collision (user key shadowing a reserved key) is a hard error so misuse
// is caught at validation/execution time rather than silently overwriting
// the reserved value.
func mergeRepoVariables(reserved map[string]string, user map[string]string, kind, repoName string) (map[string]string, error) {
	for key := range user {
		if _, exists := reserved[key]; exists {
			return nil, fmt.Errorf("repo %s (%s) variables: %q collides with reserved repo field", repoName, kind, key)
		}
	}
	for key, value := range user {
		reserved[key] = value
	}
	return reserved, nil
}
