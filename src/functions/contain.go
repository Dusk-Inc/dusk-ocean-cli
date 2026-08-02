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

// StageBuildContextForNode creates a staging directory at .ocean/stage/ containing
// the node's directory and its transitive local deps, mirroring their paths from the
// workspace root. Returns the staging root path. This generic form backs both
// StageServiceBuildContext and ContainProject.
func StageBuildContextForNode(fs afero.Fs, root string, config WorkspaceConfig, node Node, out io.Writer) (string, error) {
	return StageBuildContextForNodeAt(fs, root, config, node, filepath.Join(root, ".ocean", "stage"), out)
}

// StageBuildContextForNodeAt is StageBuildContextForNode with an explicit staging
// destination, so the no-publish dev contain path (D6) can stage a service into its
// app's jobs/ folder as a persistent compose build context instead of the transient
// .ocean/stage.
func StageBuildContextForNodeAt(fs afero.Fs, root string, config WorkspaceConfig, node Node, stagingPath string, out io.Writer) (string, error) {
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

	deps, err := CollectDependencyOrder(config, node)
	if err != nil {
		return "", err
	}

	// Stage the target node itself and each transitive dep.
	nodesToStage := append(deps, node)
	for _, n := range nodesToStage {
		_, srcPath, _, err := NodeBuildInfo(root, n)
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

// StageServiceBuildContext creates a staging directory for a service and its
// transitive deps (REQ 10.5/10.6). Delegates to StageBuildContextForNode.
func StageServiceBuildContext(fs afero.Fs, root string, config WorkspaceConfig, appName string, serviceName string, out io.Writer) (string, error) {
	serviceNode, err := MakeServiceNode(config, appName, serviceName)
	if err != nil {
		return "", err
	}
	return StageBuildContextForNode(fs, root, config, serviceNode, out)
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

// ContainServiceDev stages a service and its transitive workspace deps into the
// app's jobs/.stage/<service>/ folder as a self-contained compose build context and
// stops — it runs no `docker build`/`docker push` and never removes the staged tree.
// This is the no-publish dev contain mode (D6): the ephemeral-deps stack's
// docker-compose.<service>.yml builds and runs the service itself from this context,
// on dusk_dev_net, so a Nuclei runner can attack it as a real in-stack external.
func ContainServiceDev(cmd *cobra.Command, fs afero.Fs, appName string, serviceName string) error {
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

	serviceNode, err := MakeServiceNode(config, resolvedApp, resolvedSvc)
	if err != nil {
		return err
	}

	stagingPath := filepath.Join(root, "repos", "apps", resolvedApp, "jobs", ".stage", resolvedSvc)
	if _, err := StageBuildContextForNodeAt(fs, root, config, serviceNode, stagingPath, cmd.OutOrStdout()); err != nil {
		return err
	}

	rel, relErr := filepath.Rel(filepath.Join(root, "repos", "apps", resolvedApp, "jobs"), stagingPath)
	if relErr != nil {
		rel = stagingPath
	}
	fmt.Fprintf(cmd.OutOrStdout(),
		"staged %s/%s build context at %s (no-publish); compose build.context: ./%s\n",
		resolvedApp, resolvedSvc, stagingPath, filepath.ToSlash(rel))
	return nil
}

// resolveProjectContainerFilePath returns the absolute path to the project
// container file. Projects have no workspace-config ContainerFile/Dockerfile
// fields, so the fallback is always <stagingPath>/repos/projects/<name>/Dockerfile.
func resolveProjectContainerFilePath(stagingPath, projectName string) string {
	return filepath.Join(stagingPath, "repos", "projects", projectName, "Dockerfile")
}

// ContainProject stages the build context and runs the project's contain task.
// Mirrors ContainService but for project-kind repos, which have no app scope,
// port, or image_path fields in workspace config.
func ContainProject(cmd *cobra.Command, fs afero.Fs, projectName string) error {
	root, err := EnsureWorkspaceRoot(fs)
	if err != nil {
		return err
	}

	config, err := ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}

	projectNode, err := MakeProjectNode(config, projectName)
	if err != nil {
		return err
	}

	projectPath := filepath.Join(root, "repos", "projects", projectName)
	containTask, err := ReadRepoCommand(fs, projectPath, "contain")
	if err != nil {
		return err
	}
	if strings.TrimSpace(containTask) == "" {
		fmt.Fprintf(cmd.OutOrStdout(), "contain skipped for project %s: no contain task\n", projectName)
		return nil
	}

	_, _, buildHashPath, err := NodeBuildInfo(root, projectNode)
	if err != nil {
		return err
	}
	containHashPath := MakeContainHashPath(buildHashPath)

	newHash, err := CalcNodeTreeHash(fs, root, config, projectNode)
	if err != nil {
		return err
	}
	prevHash, hasPrev, err := ReadHashFile(fs, containHashPath)
	if err != nil {
		return err
	}
	if hasPrev && prevHash == newHash {
		fmt.Fprintf(cmd.OutOrStdout(), "contain skipped for project %s: no changes\n", projectName)
		return SetManifestContainHash(fs, root, ProjectKey(projectName), newHash)
	}

	stagingPath, err := StageBuildContextForNode(fs, root, config, projectNode, cmd.OutOrStdout())
	if err != nil {
		return err
	}

	// Projects have no port, image_path, or container_file fields in workspace
	// config. Pass empty strings for port/image_path; fall back to the staged
	// project Dockerfile for container_file. Unmatched tokens in the task are
	// left verbatim (see substituteOceanPlaceholders).
	containerFile := resolveProjectContainerFilePath(stagingPath, projectName)
	task := substituteOceanPlaceholders(containTask, projectName, "", "", containerFile)

	// Run the contain task from the staged project directory so tasks that
	// assume cwd == project root (e.g. `docker build ... .`, `node -p
	// "require('./package.json')"`) work naturally. Deps, if any, remain
	// accessible at ../../libs/... relative to cwd.
	stagedProjectPath := filepath.Join(stagingPath, "repos", "projects", projectName)
	execCmd := exec.Command("bash", "-lc", task)
	execCmd.Dir = stagedProjectPath
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
	return SetManifestContainHash(fs, root, ProjectKey(projectName), newHash)
}
