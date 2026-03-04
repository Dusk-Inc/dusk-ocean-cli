package functions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// ManifestEntry records the hash state of a single repository.
//
// Schema (.ocean/manifest.json):
//
//	{
//	  "repos": {
//	    "<node-key>": {
//	      "kind":      "<NodeKind>",        // e.g. "service", "global-lib", "app-lib", "project", "app-test"
//	      "app":       "<app-name>",        // owning app; omitted for global libs and projects
//	      "name":      "<repo-name>",       // repo name within its kind/app scope
//	      "hash":      "<sha256-hex>",      // directory content hash at last hash run
//	      "dirty":     true | false,        // true when hash differs from the previous recorded hash
//	      "build_run": true | false,        // true when build completed successfully after the current hash was set
//	      "check_run": true | false,        // true when check completed successfully after the current hash was set
//	      "hashed_at": "<RFC3339-UTC>"      // timestamp of the last hash computation
//	    },
//	    ...
//	  }
//	}
//
// Node keys use the format produced by nodeKey(): "service:<app>:<name>",
// "lib:global:<name>", "lib:app:<app>:<name>", "project:<name>", "test:<app>:<name>".
type ManifestEntry struct {
	Kind     string `json:"kind"`
	App      string `json:"app,omitempty"`
	Name     string `json:"name"`
	Hash     string `json:"hash"`
	Dirty    bool   `json:"dirty"`
	BuildRun bool   `json:"build_run"`
	CheckRun bool   `json:"check_run"`
	HashedAt string `json:"hashed_at"`
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

// HashAllRepos computes directory hashes for every registered repository and updates
// .ocean/manifest.json (REQ 12.1/12.7).
func HashAllRepos(cmd *cobra.Command, fs afero.Fs, root string, config WorkspaceConfig) error {
	ignorePatterns, err := ReadGitignorePatterns(fs, root)
	if err != nil {
		return err
	}
	m, err := ReadManifest(fs, root)
	if err != nil {
		return err
	}
	nodes := CollectWorkspaceNodes(config)
	for _, node := range nodes {
		if err := hashNode(fs, root, node, &m, ignorePatterns); err != nil {
			return err
		}
	}
	if err := WriteManifest(fs, root, m); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "hashed %d repos\n", len(nodes))
	return nil
}

// HashSingleRepo computes the hash for one repo by name and updates the manifest (REQ 12.2).
// Uses ResolveTargetByName for disambiguation when the same name appears in multiple scopes.
func HashSingleRepo(cmd *cobra.Command, fs afero.Fs, root string, config WorkspaceConfig, targetName string) error {
	target, err := ResolveTargetByName(config, root, targetName)
	if err != nil {
		return err
	}
	node, err := targetToNode(config, target)
	if err != nil {
		return err
	}
	ignorePatterns, err := ReadGitignorePatterns(fs, root)
	if err != nil {
		return err
	}
	m, err := ReadManifest(fs, root)
	if err != nil {
		return err
	}
	if err := hashNode(fs, root, node, &m, ignorePatterns); err != nil {
		return err
	}
	if err := WriteManifest(fs, root, m); err != nil {
		return err
	}
	key := nodeKey(node)
	entry := m.Repos[key]
	status := "clean"
	if entry.Dirty {
		status = "dirty"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "hashed %s: %s (%s)\n", key, entry.Hash[:8], status)
	return nil
}

// SetManifestBuildRun marks build_run=true for the given repo key in the manifest (REQ 12.5).
// No-op if the manifest is absent or the key is not present.
func SetManifestBuildRun(fs afero.Fs, root string, key string) error {
	return updateManifestEntry(fs, root, key, func(e ManifestEntry) ManifestEntry {
		e.BuildRun = true
		return e
	})
}

// SetManifestCheckRun marks check_run=true for the given repo key in the manifest (REQ 12.6).
// No-op if the manifest is absent or the key is not present.
func SetManifestCheckRun(fs afero.Fs, root string, key string) error {
	return updateManifestEntry(fs, root, key, func(e ManifestEntry) ManifestEntry {
		e.CheckRun = true
		return e
	})
}

// hashNode computes and upserts one node's entry in the provided manifest.
// The caller is responsible for calling WriteManifest to persist changes.
// If the repo directory does not exist on disk the node is silently skipped.
func hashNode(fs afero.Fs, root string, node Node, m *Manifest, ignorePatterns []string) error {
	key := nodeKey(node)
	_, srcPath, _, err := NodeBuildInfo(root, node)
	if err != nil {
		return err
	}
	if !DirExists(fs, srcPath) {
		return nil
	}
	newHash, err := CalcDirHash(fs, srcPath, ignorePatterns)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	prev, hasPrev := m.Repos[key]
	entry := ManifestEntry{
		Kind:     string(node.Kind),
		App:      node.App,
		Name:     node.Name,
		Hash:     newHash,
		HashedAt: now,
	}
	if !hasPrev || prev.Hash != newHash {
		// New entry or hash changed: mark dirty and reset run flags (REQ 12.3/12.7).
		entry.Dirty = true
		entry.BuildRun = false
		entry.CheckRun = false
	} else {
		// Hash unchanged: preserve run flags (REQ 12.4).
		entry.Dirty = false
		entry.BuildRun = prev.BuildRun
		entry.CheckRun = prev.CheckRun
	}
	m.Repos[key] = entry
	return nil
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
