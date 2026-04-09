package functions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
	"github.com/spf13/afero"
)

// starterConfigFile mirrors the README's "auto-generated starter
// ocean.config.json" shape: name + language + type + an explicit empty
// task block. The developer fills the task strings in afterward.
type starterConfigFile struct {
	Name     string            `json:"name"`
	Language string            `json:"language"`
	Type     string            `json:"type"`
	Tasks    starterConfigTask `json:"tasks"`
}

type starterConfigTask struct {
	Build     string `json:"build"`
	Test      string `json:"test"`
	Install   string `json:"install"`
	Add       string `json:"add"`
	Uninstall string `json:"uninstall"`
	Contain   string `json:"contain"`
	Run       string `json:"run"`
}

// WriteStarterRepoConfig drops the canonical empty ocean.config.json into
// the given repo directory. The caller is expected to have already
// confirmed the file does not exist (adopt/register both check), but the
// helper double-checks so it never silently overwrites a developer's
// existing configuration.
func WriteStarterRepoConfig(fs afero.Fs, repoPath string, name string, kind string) error {
	configPath := filepath.Join(repoPath, "ocean.config.json")
	if _, err := fs.Stat(configPath); err == nil {
		return fmt.Errorf("ocean.config.json already exists at %s", repoPath)
	} else if !os.IsNotExist(err) {
		return err
	}

	cfg := starterConfigFile{
		Name:     name,
		Language: "",
		Type:     starterConfigTypeFromKind(kind),
	}
	payload, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	if err := fs.MkdirAll(repoPath, 0o755); err != nil {
		return err
	}
	return afero.WriteFile(fs, configPath, payload, 0o644)
}

// starterConfigTypeFromKind maps adopt/register kinds to the `type` field
// written into ocean.config.json. The CLI uses these as classification
// hints for downstream commands.
func starterConfigTypeFromKind(kind string) string {
	switch kind {
	case tokens.RepoKindProject:
		return "project"
	case tokens.RepoKindLibrary:
		return "library"
	case tokens.RepoKindApp:
		return "app"
	case tokens.RepoKindService:
		return "service"
	}
	return ""
}
