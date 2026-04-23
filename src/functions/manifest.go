package functions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// ManifestEntry records per-operation hashes for a single repository.
// Each operation (build, check, contain, publish) stores the dependency-tree
// hash at the time it last succeeded. To determine whether an operation is
// stale, compute the current tree hash and compare it to the stored value.
//
// Schema (.ocean/manifest.json):
//
//	{
//	  "repos": {
//	    "<node-key>": {
//	      "kind":         "<NodeKind>",     // e.g. "service", "global-lib", "app-lib", "project", "app-test"
//	      "app":          "<app-name>",     // owning app; omitted for global libs and projects
//	      "name":         "<repo-name>",    // repo name within its kind/app scope
//	      "build_hash":   "<sha256-hex>",   // tree hash at last successful build
//	      "check_hash":   "<sha256-hex>",   // tree hash at last successful check
//	      "contain_hash": "<sha256-hex>",   // tree hash at last successful contain
//	      "publish_hash": "<sha256-hex>"    // tree hash at last successful publish
//	    },
//	    ...
//	  }
//	}
//
// Node keys use the format produced by nodeKey(): "service:<app>:<name>",
// "lib:global:<name>", "lib:app:<app>:<name>", "project:<name>", "test:<app>:<name>".
type ManifestEntry struct {
	Kind        string `json:"kind"`
	App         string `json:"app,omitempty"`
	Name        string `json:"name"`
	BuildHash   string `json:"build_hash"`
	CheckHash   string `json:"check_hash"`
	ContainHash string `json:"contain_hash"`
	PublishHash string `json:"publish_hash,omitempty"`
}

// Manifest is the in-memory representation of .ocean/manifest.json.
type Manifest struct {
	Repos map[string]ManifestEntry `json:"repos"`
}

// ManifestPath returns the absolute path to .ocean/manifest.json.
func ManifestPath(root string) string {
	return filepath.Join(root, ".ocean", "manifest.json")
}

// ReadManifest reads .ocean/manifest.json. Returns an empty manifest if the file is absent.
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

// WriteManifest writes the manifest atomically to .ocean/manifest.json.
// It writes to a .tmp file first, then renames, preventing partial reads on concurrent access.
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

// HashAllRepos ensures manifest entries exist for every registered repository (REQ 12.1/12.7).
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

// HashSingleRepo ensures a manifest entry exists for one repo by name (REQ 12.2).
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

// SetManifestBuildHash stores the dependency-tree hash for a successful build (REQ 12.5).
// No-op if the manifest is absent or the key is not present.
func SetManifestBuildHash(fs afero.Fs, root string, key string, hash string) error {
	return updateManifestEntry(fs, root, key, func(e ManifestEntry) ManifestEntry {
		e.BuildHash = hash
		return e
	})
}

// SetManifestCheckHash stores the dependency-tree hash for a successful check (REQ 12.6).
// No-op if the manifest is absent or the key is not present.
func SetManifestCheckHash(fs afero.Fs, root string, key string, hash string) error {
	return updateManifestEntry(fs, root, key, func(e ManifestEntry) ManifestEntry {
		e.CheckHash = hash
		return e
	})
}

// SetManifestContainHash stores the dependency-tree hash for a successful contain (REQ 12.8).
// No-op if the manifest is absent or the key is not present.
func SetManifestContainHash(fs afero.Fs, root string, key string, hash string) error {
	return updateManifestEntry(fs, root, key, func(e ManifestEntry) ManifestEntry {
		e.ContainHash = hash
		return e
	})
}

// SetManifestPublishHash stores the dependency-tree hash for a successful publish.
// No-op if the manifest is absent or the key is not present.
func SetManifestPublishHash(fs afero.Fs, root string, key string, hash string) error {
	return updateManifestEntry(fs, root, key, func(e ManifestEntry) ManifestEntry {
		e.PublishHash = hash
		return e
	})
}

// ensureManifestEntry creates a manifest entry for a node if one does not already exist.
// Existing entries are left unchanged.
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

// updateManifestEntry reads the manifest, applies fn to the named entry, and writes it back.
// If the manifest is absent or the key is not present the function returns nil (no-op).
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

// targetToNode converts a resolved Target to a Node, looking up dep lists from config.
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
