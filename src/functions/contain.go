package functions

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// ResolveContainTarget resolves the app and service names for the contain command.
// If appName is provided, it resolves directly within that app (REQ 10.3).
// If appName is empty, it scans all apps for the service name (REQ 10.1/10.2).
func ResolveContainTarget(config WorkspaceConfig, appName string, serviceName string) (string, string, error) {
	if appName != "" {
		appIdx := FindAppIndex(config, appName)
		if appIdx == -1 {
			return "", "", fmt.Errorf("app not found: %s", appName)
		}
		if FindServiceIndex(config.Apps[appIdx], serviceName) == -1 {
			return "", "", fmt.Errorf("service not found: %s in app %s", serviceName, appName)
		}
		return appName, serviceName, nil
	}

	var matches []string
	for _, app := range config.Apps {
		if FindServiceIndex(app, serviceName) != -1 {
			matches = append(matches, app.Name)
		}
	}
	switch len(matches) {
	case 0:
		return "", "", fmt.Errorf("service not found: %s", serviceName)
	case 1:
		return matches[0], serviceName, nil
	default:
		return "", "", fmt.Errorf("ambiguous service name: %s appears in apps %s; use --app to specify", serviceName, strings.Join(matches, ", "))
	}
}

// ReadOceanIgnorePatterns reads .oceanignore from the workspace root.
// Returns the patterns and true if found; returns nil and false if the file is absent.
func ReadOceanIgnorePatterns(fs afero.Fs, root string) ([]string, bool) {
	path := filepath.Join(root, ".oceanignore")
	f, err := fs.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()

	var patterns []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns, true
}

// ReadOceanIncludePaths reads .oceaninclude from the workspace root.
// Returns the paths and true if found; returns nil and false if the file is absent.
func ReadOceanIncludePaths(fs afero.Fs, root string) ([]string, bool) {
	path := filepath.Join(root, ".oceaninclude")
	f, err := fs.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()

	var paths []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		paths = append(paths, line)
	}
	return paths, true
}

// StageServiceBuildContext creates a staging directory at .ocean/stage/ containing
// the service directory and its transitive local deps, mirroring their paths from the
// workspace root. Returns the staging root path (REQ 10.5/10.6).
func StageServiceBuildContext(fs afero.Fs, root string, config WorkspaceConfig, appName string, serviceName string, out io.Writer) (string, error) {
	stagingPath := filepath.Join(root, ".ocean", "stage")

	if err := fs.RemoveAll(stagingPath); err != nil {
		return "", fmt.Errorf("failed to clear staging directory: %w", err)
	}
	if err := fs.MkdirAll(stagingPath, 0o755); err != nil {
		return "", fmt.Errorf("failed to create staging directory: %w", err)
	}

	ignorePatterns, found := ReadOceanIgnorePatterns(fs, root)
	if !found {
		fmt.Fprintf(out, ".oceanignore not found; proceeding without ignore patterns\n")
	}

	serviceNode, err := MakeServiceNode(config, appName, serviceName)
	if err != nil {
		return "", err
	}

	deps, err := CollectDependencyOrder(config, serviceNode)
	if err != nil {
		return "", err
	}

	// Stage the service itself and each transitive dep.
	nodesToStage := append(deps, serviceNode)
	for _, node := range nodesToStage {
		_, srcPath, _, err := NodeBuildInfo(root, node)
		if err != nil {
			return "", err
		}
		relPath, err := filepath.Rel(root, srcPath)
		if err != nil {
			return "", err
		}
		dstPath := filepath.Join(stagingPath, relPath)
		if err := copyDir(fs, srcPath, dstPath, ignorePatterns); err != nil {
			return "", fmt.Errorf("failed to stage %s: %w", relPath, err)
		}
	}

	// Copy .oceaninclude files to staging root.
	includePaths, found := ReadOceanIncludePaths(fs, root)
	if !found {
		fmt.Fprintf(out, ".oceaninclude not found; no workspace files will be copied to staging root\n")
	}
	for _, includePath := range includePaths {
		srcFile := filepath.Join(root, includePath)
		dstFile := filepath.Join(stagingPath, filepath.Base(includePath))
		if err := copyFile(fs, srcFile, dstFile); err != nil {
			return "", fmt.Errorf("failed to copy include file %s: %w", includePath, err)
		}
	}

	return stagingPath, nil
}

// copyDir copies the directory at src to dst, excluding files matching ignorePatterns.
func copyDir(fs afero.Fs, src string, dst string, ignorePatterns []string) error {
	return afero.Walk(fs, src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		if relPath != "." && shouldIgnore(relPath, info.IsDir(), ignorePatterns) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return fs.MkdirAll(dstPath, info.Mode())
		}

		return copyFile(fs, path, dstPath)
	})
}

// copyFile copies a single file from src to dst, creating parent directories as needed.
func copyFile(fs afero.Fs, src string, dst string) error {
	if err := fs.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	srcFile, err := fs.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	info, err := fs.Stat(src)
	if err != nil {
		return err
	}

	dstFile, err := fs.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// ContainService stages the build context and runs docker build + push for a service (REQ 10).
func ContainService(cmd *cobra.Command, fs afero.Fs, appName string, serviceName string) error {
	root, err := EnsureWorkspaceRoot(fs)
	if err != nil {
		return err
	}

	config, err := ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}

	// REQ 10.1/10.2/10.3: resolve app and service.
	resolvedApp, resolvedSvc, err := ResolveContainTarget(config, appName, serviceName)
	if err != nil {
		return err
	}

	imageName, err := ServiceImageReference(fs, resolvedApp, resolvedSvc)
	if err != nil {
		return err
	}

	// REQ 10.5/10.6: stage build context.
	stagingPath, err := StageServiceBuildContext(fs, root, config, resolvedApp, resolvedSvc, cmd.OutOrStdout())
	if err != nil {
		return err
	}

	// Resolve Dockerfile path.
	dockerfilePath := resolveDockerfilePath(config, root, stagingPath, resolvedApp, resolvedSvc)

	// REQ 10.1: docker build.
	buildCmd := exec.Command("docker", "build", "-t", imageName, "-f", dockerfilePath, ".")
	buildCmd.Dir = stagingPath
	buildCmd.Stdout = cmd.OutOrStdout()
	buildCmd.Stderr = cmd.ErrOrStderr()
	buildCmd.Stdin = cmd.InOrStdin()
	if err := buildCmd.Run(); err != nil {
		// REQ 10.4: surface build error, do not push.
		_ = fs.RemoveAll(stagingPath)
		return err
	}

	// REQ 10.1: docker push.
	pushCmd := exec.Command("docker", "push", imageName)
	pushCmd.Stdout = cmd.OutOrStdout()
	pushCmd.Stderr = cmd.ErrOrStderr()
	pushCmd.Stdin = cmd.InOrStdin()
	pushErr := pushCmd.Run()

	_ = fs.RemoveAll(stagingPath)
	return pushErr
}

// resolveDockerfilePath returns the absolute path to the service Dockerfile.
// Uses WorkspaceService.Dockerfile if set, otherwise defaults to Dockerfile in the service directory.
func resolveDockerfilePath(config WorkspaceConfig, root string, stagingPath string, appName string, serviceName string) string {
	appIdx := FindAppIndex(config, appName)
	if appIdx != -1 {
		svcIdx := FindServiceIndex(config.Apps[appIdx], serviceName)
		if svcIdx != -1 && strings.TrimSpace(config.Apps[appIdx].Services[svcIdx].Dockerfile) != "" {
			return filepath.Join(root, config.Apps[appIdx].Services[svcIdx].Dockerfile)
		}
	}
	relServicePath := filepath.Join("repos", "apps", appName, "services", serviceName)
	return filepath.Join(stagingPath, relServicePath, "Dockerfile")
}
