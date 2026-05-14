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

func readContainCommand(targetPath string) (string, error) {
	return ReadRepoCommand(afero.NewOsFs(), targetPath, "contain")
}

func resolveImagePath(fs afero.Fs, config WorkspaceConfig, appName, serviceName string) string {
	appIdx := FindAppIndex(config, appName)
	if appIdx != -1 {
		svcIdx := FindServiceIndex(config.Apps[appIdx], serviceName)
		if svcIdx != -1 {
			if ip := strings.TrimSpace(config.Apps[appIdx].Services[svcIdx].ImagePath); ip != "" {
				return ip
			}
		}
	}
	img, _ := ServiceImageReference(fs, appName, serviceName)
	return img
}

func resolveContainerFilePath(config WorkspaceConfig, root, stagingPath, appName, serviceName string) string {
	appIdx := FindAppIndex(config, appName)
	if appIdx != -1 {
		svcIdx := FindServiceIndex(config.Apps[appIdx], serviceName)
		if svcIdx != -1 {
			svc := config.Apps[appIdx].Services[svcIdx]
			value := strings.TrimSpace(svc.ContainerFile)
			if value == "" {
				value = strings.TrimSpace(svc.Dockerfile)
			}
			if value != "" {
				if filepath.Base(value) == value {
					return filepath.Join(root, "repos", "containers", value)
				}
				return filepath.Join(root, value)
			}
		}
	}
	relServicePath := filepath.Join("repos", "apps", appName, "services", serviceName)
	return filepath.Join(stagingPath, relServicePath, "Dockerfile")
}

func substituteOceanPlaceholders(task, serviceName, port, imagePath, containerFile string) string {
	task = strings.ReplaceAll(task, "{{ocean:service_name}}", serviceName)
	task = strings.ReplaceAll(task, "{{ocean:port}}", port)
	task = strings.ReplaceAll(task, "{{ocean:image_path}}", imagePath)
	task = strings.ReplaceAll(task, "{{ocean:container_file}}", containerFile)
	return task
}

func ContainService(cmd *cobra.Command, fs afero.Fs, appName string, serviceName string) error {
	root, err := EnsureWorkspaceRoot(fs)
	if err != nil {
		return err
	}

	config, err := ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}

	resolvedApp, resolvedSvc, err := ResolveContainTarget(config, appName, serviceName)
	if err != nil {
		return err
	}

	servicePath := filepath.Join(root, "repos", "apps", resolvedApp, "services", resolvedSvc)
	containTask, err := readContainCommand(servicePath)
	if err != nil {
		return err
	}
	if strings.TrimSpace(containTask) == "" {
		fmt.Fprintf(cmd.OutOrStdout(), "contain skipped for service %s/%s: no contain task\n", resolvedApp, resolvedSvc)
		return nil
	}

	serviceNode, err := MakeServiceNode(config, resolvedApp, resolvedSvc)
	if err != nil {
		return err
	}
	_, _, buildHashPath, err := NodeBuildInfo(root, serviceNode)
	if err != nil {
		return err
	}
	containHashPath := MakeContainHashPath(buildHashPath)

	newHash, err := CalcContainTreeHash(fs, root, config, resolvedApp, resolvedSvc)
	if err != nil {
		return err
	}
	prevHash, hasPrev, err := ReadHashFile(fs, containHashPath)
	if err != nil {
		return err
	}

	if hasPrev && prevHash == newHash {
		fmt.Fprintf(cmd.OutOrStdout(), "contain skipped for service %s/%s: no changes\n", resolvedApp, resolvedSvc)
		key := ServiceKey(resolvedApp, resolvedSvc)
		return SetManifestContainHash(fs, root, key, newHash)
	}

	stagingPath, err := StageServiceBuildContext(fs, root, config, resolvedApp, resolvedSvc, cmd.OutOrStdout())
	if err != nil {
		return err
	}

	containerFile := resolveContainerFilePath(config, root, stagingPath, resolvedApp, resolvedSvc)
	imagePath := resolveImagePath(fs, config, resolvedApp, resolvedSvc)

	port := ""
	appIdx := FindAppIndex(config, resolvedApp)
	if appIdx != -1 {
		svcIdx := FindServiceIndex(config.Apps[appIdx], resolvedSvc)
		if svcIdx != -1 {
			port = config.Apps[appIdx].Services[svcIdx].Port
		}
	}

	task := substituteOceanPlaceholders(containTask, resolvedSvc, port, imagePath, containerFile)

	execCmd := exec.Command("bash", "-lc", task)
	execCmd.Dir = stagingPath
	execCmd.Stdout = cmd.OutOrStdout()
	execCmd.Stderr = cmd.ErrOrStderr()
	execCmd.Stdin = cmd.InOrStdin()
	runErr := execCmd.Run()

	_ = fs.RemoveAll(stagingPath)

	if runErr != nil {
		return runErr
	}

	if err := WriteHashFile(fs, containHashPath, newHash); err != nil {
		return err
	}
	key := ServiceKey(resolvedApp, resolvedSvc)
	return SetManifestContainHash(fs, root, key, newHash)
}
