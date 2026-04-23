package functions

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

func CalcDirHash(fs afero.Fs, root string, ignorePatterns []string) (string, error) {
	hasher := sha256.New()
	err := afero.Walk(fs, root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(root, path)
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
		if info.IsDir() {
			return nil
		}
		fmt.Fprintf(hasher, "%s|%d|%d;", relPath, info.Size(), info.ModTime().UnixNano())
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func shouldIgnore(relPath string, isDir bool, patterns []string) bool {
	ignored := false
	for _, raw := range patterns {
		pattern := strings.TrimSpace(raw)
		if pattern == "" || strings.HasPrefix(pattern, "#") {
			continue
		}
		negated := strings.HasPrefix(pattern, "!")
		if negated {
			pattern = strings.TrimSpace(pattern[1:])
		}
		if pattern == "" {
			continue
		}
		if matchesIgnorePattern(relPath, isDir, pattern) {
			ignored = !negated
		}
	}
	return ignored
}

func matchesIgnorePattern(relPath string, isDir bool, pattern string) bool {
	pattern = filepath.ToSlash(pattern)
	if after, ok :=strings.CutPrefix(pattern, "/"); ok  {
		pattern = after
	}
	dirOnly := strings.HasSuffix(pattern, "/")
	if dirOnly {
		pattern = strings.TrimSuffix(pattern, "/")
	}
	if pattern == "" {
		return false
	}
	if dirOnly {
		if relPath == pattern || strings.HasPrefix(relPath, pattern+"/") {
			return true
		}
		if !isDir {
			return false
		}
	}
	if strings.Contains(pattern, "/") {
		matched, _ := path.Match(pattern, relPath)
		return matched
	}
	if match, _ := path.Match(pattern, path.Base(relPath)); match {
		return true
	}
	for _, segment := range strings.Split(relPath, "/") {
		if segment == "" {
			continue
		}
		if match, _ := path.Match(pattern, segment); match {
			return true
		}
	}
	return false
}

// CalcRepoHash computes the directory hash for a repository, using .gitignore
// patterns from the workspace root AND the repo root for ignore filtering. This
// is the single entry point for computing repo hashes across build, check, and
// manifest operations.
func CalcRepoHash(fs afero.Fs, root string, repoPath string) (string, error) {
	ignorePatterns, err := CollectRepoIgnorePatterns(fs, root, repoPath)
	if err != nil {
		return "", err
	}
	return CalcDirHash(fs, repoPath, ignorePatterns)
}

// CalcNodeTreeHash computes a combined hash of a node and all its transitive
// dependency directories. Ignore patterns are collected per-node from both the
// workspace-root .gitignore and the node's own repo-local .gitignore, so each
// repo's build artifacts (e.g. dist/, coverage/) are excluded from its hash.
// This is the single entry point for computing dependency-tree hashes used by
// the manifest and operation skip logic.
func CalcNodeTreeHash(fs afero.Fs, root string, config WorkspaceConfig, node Node) (string, error) {
	deps, err := CollectDependencyOrder(config, node)
	if err != nil {
		return "", err
	}
	nodesToHash := append(deps, node)

	combined := sha256.New()
	for _, n := range nodesToHash {
		_, srcPath, _, err := NodeBuildInfo(root, n)
		if err != nil {
			return "", err
		}
		ignorePatterns, err := CollectRepoIgnorePatterns(fs, root, srcPath)
		if err != nil {
			return "", err
		}
		h, err := CalcDirHash(fs, srcPath, ignorePatterns)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(combined, "%s:%s\n", nodeKey(n), h)
	}
	return hex.EncodeToString(combined.Sum(nil)), nil
}

// CalcContainTreeHash computes a dependency-tree hash for a service.
// Convenience wrapper around CalcNodeTreeHash for the contain command (REQ 10.7/10.8).
func CalcContainTreeHash(fs afero.Fs, root string, config WorkspaceConfig, appName, serviceName string) (string, error) {
	serviceNode, err := MakeServiceNode(config, appName, serviceName)
	if err != nil {
		return "", err
	}
	return CalcNodeTreeHash(fs, root, config, serviceNode)
}

func ReadHashFile(fs afero.Fs, path string) (string, bool, error) {
	payload, err := afero.ReadFile(fs, path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return strings.TrimSpace(string(payload)), true, nil
}

func WriteHashFile(fs afero.Fs, path string, value string) error {
	if err := fs.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return afero.WriteFile(fs, path, []byte(value+"\n"), 0o644)
}

func MakeBuildHashPath(checkHashPath string) string {
	parts := strings.Split(filepath.Clean(checkHashPath), string(filepath.Separator))
	for i, part := range parts {
		if part == "check" {
			parts[i] = "build"
			break
		}
	}
	return filepath.Join(parts...)
}

func MakeContainHashPath(buildHashPath string) string {
	parts := strings.Split(filepath.Clean(buildHashPath), string(filepath.Separator))
	for i, part := range parts {
		if part == "build" {
			parts[i] = "contain"
			break
		}
	}
	return filepath.Join(parts...)
}

func MakePublishHashPath(buildHashPath string) string {
	parts := strings.Split(filepath.Clean(buildHashPath), string(filepath.Separator))
	for i, part := range parts {
		if part == "build" {
			parts[i] = "publish"
			break
		}
	}
	return filepath.Join(parts...)
}

func MakeHashPath(root string, kind string, parts ...string) string {
	if len(parts) == 0 {
		return filepath.Join(root, ".ocean", "hashes", "build", kind, "build.hash")
	}
	fileName := parts[len(parts)-1] + ".hash"
	dirParts := append([]string{root, ".ocean", "hashes", "build", kind}, parts[:len(parts)-1]...)
	dir := filepath.Join(dirParts...)
	return filepath.Join(dir, fileName)
}

func MakeCheckHashPath(root string, kind string, parts ...string) string {
	if len(parts) == 0 {
		return filepath.Join(root, ".ocean", "hashes", "check", kind, "check.hash")
	}
	fileName := parts[len(parts)-1] + ".hash"
	dirParts := append([]string{root, ".ocean", "hashes", "check", kind}, parts[:len(parts)-1]...)
	dir := filepath.Join(dirParts...)
	return filepath.Join(dir, fileName)
}

func CleanupStaleHashes(fs afero.Fs, cmd *cobra.Command, root string) error {
	hashRoot := filepath.Join(root, ".ocean", "hashes")
	info, err := fs.Stat(hashRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", hashRoot)
	}

	var stale []string
	err = afero.Walk(fs, hashRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".hash") {
			return nil
		}
		if hashTargetExists(fs, root, hashRoot, path) {
			return nil
		}
		stale = append(stale, path)
		return nil
	})
	if err != nil {
		return err
	}
	if len(stale) == 0 {
		return nil
	}
	sort.Strings(stale)
	for _, path := range stale {
		if err := fs.Remove(path); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "removed stale hash: %s\n", path)
	}
	return nil
}

func ClearHashes(fs afero.Fs, cmd *cobra.Command, root string) error {
	hashRoot := filepath.Join(root, ".ocean", "hashes")
	info, err := fs.Stat(hashRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", hashRoot)
	}

	paths := []string{
		filepath.Join(hashRoot, "build"),
		filepath.Join(hashRoot, "check"),
	}
	for _, hashPath := range paths {
		if err := fs.RemoveAll(hashPath); err != nil {
			return err
		}
	}
	fmt.Fprintln(cmd.OutOrStdout(), "cleared build/check hashes")
	return nil
}

func hashTargetExists(fs afero.Fs, root string, hashRoot string, hashPath string) bool {
	rel, err := filepath.Rel(hashRoot, hashPath)
	if err != nil {
		return false
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) < 3 {
		return true
	}
	kind := parts[1]
	sub := parts[2:]
	name := strings.TrimSuffix(sub[len(sub)-1], ".hash")

	switch kind {
	case "services":
		if len(sub) < 2 {
			return true
		}
		appName := sub[0]
		target := filepath.Join(root, "repos", "apps", appName, "services", name)
		return DirExists(fs, target)
	case "libs":
		if len(sub) < 2 {
			return true
		}
		if sub[0] == "global" {
			if len(sub) < 2 {
				return true
			}
			target := filepath.Join(root, "repos", "libs", name)
			return DirExists(fs, target)
		}
		appName := sub[0]
		target := filepath.Join(root, "repos", "apps", appName, "libs", name)
		return DirExists(fs, target)
	case "projects":
		if len(sub) < 1 {
			return true
		}
		target := filepath.Join(root, "repos", "projects", name)
		return DirExists(fs, target)
	default:
		return true
	}
}
