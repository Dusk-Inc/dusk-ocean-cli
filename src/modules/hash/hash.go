package hash

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
		return dirExists(fs, target)
	case "libs":
		if len(sub) < 2 {
			return true
		}
		if sub[0] == "global" {
			if len(sub) < 2 {
				return true
			}
			target := filepath.Join(root, "repos", "libs", name)
			return dirExists(fs, target)
		}
		appName := sub[0]
		target := filepath.Join(root, "repos", "apps", appName, "libs", name)
		return dirExists(fs, target)
	case "projects":
		if len(sub) < 1 {
			return true
		}
		target := filepath.Join(root, "repos", "projects", name)
		return dirExists(fs, target)
	default:
		return true
	}
}

func dirExists(fs afero.Fs, path string) bool {
	info, err := fs.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
