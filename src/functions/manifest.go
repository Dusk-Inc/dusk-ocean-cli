package functions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

type ManifestEntry struct {
	Kind        string `json:"kind"`
	App         string `json:"app,omitempty"`
	Name        string `json:"name"`
	BuildHash   string `json:"build_hash"`
	CheckHash   string `json:"check_hash"`
	ContainHash string `json:"contain_hash"`
	PublishHash string `json:"publish_hash,omitempty"`
}

type Manifest struct {
	Repos map[string]ManifestEntry `json:"repos"`
}

func ManifestPath(root string) string {
	return filepath.Join(root, ".ocean", "manifest.json")
}

func ReadManifest(fs afero.Fs, root string) (Manifest, error) {
	path := ManifestPath(root)
	data, err := afero.ReadFile(fs, path)
	if err != nil {
		if os.IsNotExist(err) {
			return Manifest{Repos: map[string]ManifestEntry{}}, nil
		}
		return Manifest{}, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("manifest parse error: %w", err)
	}
	if m.Repos == nil {
		m.Repos = map[string]ManifestEntry{}
	}
	return m, nil
}

func WriteManifest(fs afero.Fs, root string, m Manifest) error {
	path := ManifestPath(root)
	if err := fs.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "    ")
	if err != nil {
		return err
	}
	tmpPath := path + ".tmp"
	if err := afero.WriteFile(fs, tmpPath, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return fs.Rename(tmpPath, path)
}

func HashAllRepos(cmd *cobra.Command, fs afero.Fs, root string, config WorkspaceConfig) error {
	m, err := ReadManifest(fs, root)
	if err != nil {
		return err
	}
	nodes := CollectWorkspaceNodes(config)
	for _, node := range nodes {
		ensureManifestEntry(node, &m)
	}
	if err := WriteManifest(fs, root, m); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "hashed %d repos\n", len(nodes))
	return nil
}

func HashSingleRepo(cmd *cobra.Command, fs afero.Fs, root string, config WorkspaceConfig, targetName string) error {
	target, err := ResolveTargetByName(config, root, targetName)
	if err != nil {
		return err
	}
	node, err := targetToNode(config, target)
	if err != nil {
		return err
	}
	m, err := ReadManifest(fs, root)
	if err != nil {
		return err
	}
	ensureManifestEntry(node, &m)
	if err := WriteManifest(fs, root, m); err != nil {
		return err
	}
	key := nodeKey(node)
	fmt.Fprintf(cmd.OutOrStdout(), "registered %s\n", key)
	return nil
}

func SetManifestBuildHash(fs afero.Fs, root string, key string, hash string) error {
	return updateManifestEntry(fs, root, key, func(e ManifestEntry) ManifestEntry {
		e.BuildHash = hash
		return e
	})
}

func SetManifestCheckHash(fs afero.Fs, root string, key string, hash string) error {
	return updateManifestEntry(fs, root, key, func(e ManifestEntry) ManifestEntry {
		e.CheckHash = hash
		return e
	})
}

func SetManifestContainHash(fs afero.Fs, root string, key string, hash string) error {
	return updateManifestEntry(fs, root, key, func(e ManifestEntry) ManifestEntry {
		e.ContainHash = hash
		return e
	})
}

func ensureManifestEntry(node Node, m *Manifest) {
	key := nodeKey(node)
	if _, ok := m.Repos[key]; ok {
		return
	}
	m.Repos[key] = ManifestEntry{
		Kind: string(node.Kind),
		App:  node.App,
		Name: node.Name,
	}
}

func updateManifestEntry(fs afero.Fs, root string, key string, fn func(ManifestEntry) ManifestEntry) error {
	m, err := ReadManifest(fs, root)
	if err != nil {
		return err
	}
	entry, ok := m.Repos[key]
	if !ok {
		return nil
	}
	m.Repos[key] = fn(entry)
	return WriteManifest(fs, root, m)
}

func targetToNode(config WorkspaceConfig, target Target) (Node, error) {
	switch target.Kind {
	case TargetGlobalLib:
		return MakeGlobalLibNode(config, target.Name)
	case TargetAppLib:
		return MakeAppLibNode(config, target.App, target.Name)
	case TargetService:
		return MakeServiceNode(config, target.App, target.Name)
	case TargetProject:
		return MakeProjectNode(config, target.Name)
	case TargetTest:
		return MakeTestNode(config, target.App, target.Name)
	default:
		return Node{}, fmt.Errorf("unsupported target kind: %s", target.Kind)
	}
}
