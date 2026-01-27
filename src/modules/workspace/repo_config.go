package workspace

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
)

type RepoConfig struct {
	Name     string `json:"name"`
	Language string `json:"language"`
	Type     string `json:"type"`
	Build    string `json:"build"`
	Test     string `json:"test"`
	Install  string `json:"install"`
	Tasks    struct {
		Build   string `json:"build"`
		Test    string `json:"test"`
		Install string `json:"install"`
	} `json:"tasks"`
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
	default:
		return "", fmt.Errorf("unsupported command kind: %s", kind)
	}
}
