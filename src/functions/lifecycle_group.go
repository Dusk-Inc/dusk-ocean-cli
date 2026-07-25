package functions

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/models"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

func projectVariableContext(fs afero.Fs, root string, config WorkspaceConfig, projectName string) (VariableContext, error) {
	envValues, err := LoadEnvFile(fs, root, discardWriter{})
	if err != nil {
		return VariableContext{}, err
	}
	env := map[string]string{}
	for _, kv := range os.Environ() {
		if eq := strings.IndexByte(kv, '='); eq > 0 {
			env[kv[:eq]] = kv[eq+1:]
		}
	}
	for k, v := range envValues {
		env[k] = v
	}
	repoVars, err := BuildRepoVariables(config, tokens.RepoKindProject, "", projectName)
	if err != nil {
		return VariableContext{}, err
	}
	return VariableContext{
		Env:   env,
		Var:   LoadWorkspaceVariables(config),
		Ocean: map[string]string{},
		Repo:  repoVars,
	}, nil
}

func resolveProjectTask(fs afero.Fs, root string, config WorkspaceConfig, projectName string, task string, selection models.GroupSelection) (models.ResolvedCommand, error) {
	projectPath := filepath.Join(root, "repos", "projects", projectName)
	repoConfig, err := ReadRepoConfig(fs, projectPath)
	if err != nil {
		return models.ResolvedCommand{}, err
	}
	groups, err := ValidateOverrides(repoConfig)
	if err != nil {
		return models.ResolvedCommand{}, err
	}
	ctx, err := projectVariableContext(fs, root, config, projectName)
	if err != nil {
		return models.ResolvedCommand{}, err
	}
	return ResolveGroupCommand(task, selection, repoConfig, groups, ctx)
}

func RunProjectLifecycleTask(cmd *cobra.Command, fs afero.Fs, projectName string, task string, selection models.GroupSelection) error {
	root, err := EnsureWorkspaceRoot(fs)
	if err != nil {
		return err
	}
	config, err := ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}
	node, err := MakeProjectNode(config, projectName)
	if err != nil {
		return err
	}

	resolved, err := resolveProjectTask(fs, root, config, projectName, task, selection)
	if err != nil {
		return err
	}
	if strings.TrimSpace(resolved.Command) == "" {
		fmt.Fprintf(cmd.OutOrStdout(), "%s skipped for project %s: no %s task\n", task, projectName, task)
		return nil
	}

	repoKey := ProjectKey(projectName)
	projectPath := filepath.Join(root, "repos", "projects", projectName)

	treeHash, err := CalcNodeTreeHash(fs, root, config, node)
	if err != nil {
		return err
	}
	resolvedHash := groupSlotHash(resolved, treeHash)

	if cacheableTask(task) {
		fresh, _, err := ReadGroupCacheSlot(fs, root, repoKey, selection, task, resolvedHash)
		if err != nil {
			return err
		}
		if fresh {
			fmt.Fprintf(cmd.OutOrStdout(), "%s skipped for project %s (%s): cached, up to date\n", task, projectName, slotLabel(selection))
			return nil
		}
	}

	execCmd := exec.Command("bash", "-lc", resolved.Command)
	execCmd.Dir = projectPath
	execCmd.Stdout = cmd.OutOrStdout()
	execCmd.Stderr = cmd.ErrOrStderr()
	execCmd.Stdin = cmd.InOrStdin()
	if runErr := execCmd.Run(); runErr != nil {
		return runErr
	}

	if cacheableTask(task) {
		if err := EnsureManifestEntryForNode(fs, root, node); err != nil {
			return err
		}
	}
	if _, err := RecordCacheSlot(fs, root, repoKey, selection, task, resolvedHash); err != nil {
		return err
	}
	return nil
}

func groupSlotHash(resolved models.ResolvedCommand, treeHash string) string {
	sum := sha256.Sum256([]byte(resolved.Source + "\x00" + resolved.Command + "\x00" + treeHash))
	return hex.EncodeToString(sum[:])
}

func slotLabel(selection models.GroupSelection) string {
	if selection.IsBase {
		return "base"
	}
	return "group " + selection.Group
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }
