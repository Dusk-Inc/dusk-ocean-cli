package functions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
	"github.com/spf13/afero"
)

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

// WriteStarterRepoConfig writes a minimal ocean.config.json for a repo that carries none, refusing to overwrite an existing one.
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

// starterConfigTypeFromKind maps a repo kind onto the type a starter config declares, empty for a kind that scaffolds none.
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
	case tokens.RepoKindTest:
		return "test"
	case tokens.RepoKindInfra:
		return "infra"
	case tokens.RepoKindDocs:
		return "docs"
	}
	return ""
}
