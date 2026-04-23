package functions

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// PublishProject runs the project's publish task (e.g. `npm publish`). Unlike
// contain, publish executes against the real project directory — no staging —
// because package managers rely on the actual repo layout (package.json,
// .npmignore, etc.). Pre-flight requires a prior successful build and contain
// for the project; use skipPreflight to bypass (emergency release).
func PublishProject(cmd *cobra.Command, fs afero.Fs, projectName string, skipPreflight bool) error {
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
	publishTask, err := ReadRepoCommand(fs, projectPath, "publish")
	if err != nil {
		return err
	}
	if strings.TrimSpace(publishTask) == "" {
		fmt.Fprintf(cmd.OutOrStdout(), "publish skipped for project %s: no publish task\n", projectName)
		return nil
	}

	key := ProjectKey(projectName)

	if !skipPreflight {
		manifest, err := ReadManifest(fs, root)
		if err != nil {
			return err
		}
		entry, ok := manifest.Repos[key]
		if !ok {
			return fmt.Errorf("publish pre-flight failed for project %s: no manifest entry (run build and contain first, or pass --skip-preflight)", projectName)
		}
		if strings.TrimSpace(entry.BuildHash) == "" {
			return fmt.Errorf("publish pre-flight failed for project %s: no successful build recorded (run `dusk-ocean build project --name %s` first, or pass --skip-preflight)", projectName, projectName)
		}
		if strings.TrimSpace(entry.ContainHash) == "" {
			return fmt.Errorf("publish pre-flight failed for project %s: no successful contain recorded (run `dusk-ocean contain --project %s` first, or pass --skip-preflight)", projectName, projectName)
		}
	}

	_, _, buildHashPath, err := NodeBuildInfo(root, projectNode)
	if err != nil {
		return err
	}
	publishHashPath := MakePublishHashPath(buildHashPath)

	newHash, err := CalcNodeTreeHash(fs, root, config, projectNode)
	if err != nil {
		return err
	}
	prevHash, hasPrev, err := ReadHashFile(fs, publishHashPath)
	if err != nil {
		return err
	}
	if hasPrev && prevHash == newHash {
		fmt.Fprintf(cmd.OutOrStdout(), "publish skipped for project %s: no changes\n", projectName)
		return SetManifestPublishHash(fs, root, key, newHash)
	}

	execCmd := exec.Command("bash", "-lc", publishTask)
	execCmd.Dir = projectPath
	execCmd.Stdout = cmd.OutOrStdout()
	execCmd.Stderr = cmd.ErrOrStderr()
	execCmd.Stdin = cmd.InOrStdin()
	if runErr := execCmd.Run(); runErr != nil {
		return runErr
	}

	if err := WriteHashFile(fs, publishHashPath, newHash); err != nil {
		return err
	}
	return SetManifestPublishHash(fs, root, key, newHash)
}
