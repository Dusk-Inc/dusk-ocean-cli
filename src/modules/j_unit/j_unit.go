package j_unit

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/afero"
)

func WriteCheckResults(fs afero.Fs, root string, targetPath string, label string, startedAt time.Time, stdout []byte, stderr []byte, execErr error) error {
	resultsDir, err := makeResultsDir(root, targetPath)
	if err != nil {
		return err
	}
	if err := fs.MkdirAll(resultsDir, 0o755); err != nil {
		return err
	}
	if reportPath, err := findJUnitReport(fs, root, targetPath, startedAt); err != nil {
		return err
	} else if reportPath != "" {
		destPath := filepath.Join(resultsDir, filepath.Base(reportPath))
		if err := copyJUnitReport(fs, reportPath, destPath); err != nil {
			return err
		}
		return nil
	}
	fileName := fmtCheckResultFileName(label)
	payload, err := makeJUnitReport(label, stdout, stderr, execErr)
	if err != nil {
		return err
	}
	return afero.WriteFile(fs, filepath.Join(resultsDir, fileName), payload, 0o644)
}

func makeResultsDir(root string, targetPath string) (string, error) {
	relPath, err := filepath.Rel(root, targetPath)
	if err != nil {
		return "", err
	}
	parts := strings.Split(relPath, string(filepath.Separator))
	if len(parts) > 0 && parts[0] == "repos" {
		parts = parts[1:]
	}
	return filepath.Join(append([]string{root, ".ocean"}, parts...)...), nil
}

func findJUnitReport(fs afero.Fs, root string, targetPath string, startedAt time.Time) (string, error) {
	searchRoots := []string{
		targetPath,
		filepath.Join(root, ".ocean", "results"),
	}
	var selectedPath string
	var selectedMod time.Time
	for _, searchRoot := range searchRoots {
		info, err := fs.Stat(searchRoot)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", err
		}
		if !info.IsDir() {
			continue
		}
		err = afero.Walk(fs, searchRoot, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				if info.Name() == ".git" || info.Name() == "node_modules" || info.Name() == "vendor" || info.Name() == ".ocean" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(info.Name(), ".xml") {
				return nil
			}
			if info.ModTime().Before(startedAt) {
				return nil
			}
			isJUnit, err := isJUnitXML(fs, path)
			if err != nil {
				return err
			}
			if !isJUnit {
				return nil
			}
			if selectedPath == "" || info.ModTime().After(selectedMod) {
				selectedPath = path
				selectedMod = info.ModTime()
			}
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	return selectedPath, nil
}

func isJUnitXML(fs afero.Fs, path string) (bool, error) {
	file, err := fs.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()

	buf := make([]byte, 0, 8192)
	tmp := make([]byte, 4096)
	for len(buf) < 65536 {
		n, err := file.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			content := string(buf)
			if strings.Contains(content, "<testsuite") || strings.Contains(content, "<testsuites") {
				return true, nil
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return false, err
		}
	}
	return false, nil
}

func copyJUnitReport(fs afero.Fs, src string, dest string) error {
	if src == dest {
		return nil
	}
	if err := fs.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	in, err := fs.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := fs.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func fmtCheckResultFileName(label string) string {
	base := strings.ToLower(label)
	replacer := strings.NewReplacer(
		" ", "__",
		"/", "__",
		"\\", "__",
		":", "__",
		".", "__",
	)
	base = replacer.Replace(base)
	base = strings.Trim(base, "_")
	if base == "" {
		base = "check"
	}
	return base + ".xml"
}

func makeJUnitReport(label string, stdout []byte, stderr []byte, execErr error) ([]byte, error) {
	var payload bytes.Buffer
	payload.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	payload.WriteString("\n")
	tests := 1
	failures := 0
	if execErr != nil {
		failures = 1
	}
	payload.WriteString(fmt.Sprintf(`<testsuite name="%s" tests="%d" failures="%d" errors="0">`, escapeXML(label), tests, failures))
	payload.WriteString("\n")
	payload.WriteString(fmt.Sprintf(`<testcase name="%s">`, escapeXML(label)))
	if execErr != nil {
		payload.WriteString("\n")
		payload.WriteString(fmt.Sprintf(`<failure message="%s">`, escapeXML(execErr.Error())))
		if len(stderr) > 0 {
			payload.WriteString(escapeXML(string(stderr)))
		}
		payload.WriteString("</failure>")
		payload.WriteString("\n")
	}
	payload.WriteString("</testcase>")
	payload.WriteString("\n")
	payload.WriteString("<system-out>")
	if len(stdout) > 0 {
		payload.WriteString(escapeXML(string(stdout)))
	}
	payload.WriteString("</system-out>")
	payload.WriteString("\n")
	payload.WriteString("<system-err>")
	if len(stderr) > 0 {
		payload.WriteString(escapeXML(string(stderr)))
	}
	payload.WriteString("</system-err>")
	payload.WriteString("\n")
	payload.WriteString("</testsuite>")
	payload.WriteString("\n")
	return payload.Bytes(), nil
}

func escapeXML(value string) string {
	var buf bytes.Buffer
	if err := xml.EscapeText(&buf, []byte(value)); err != nil {
		return ""
	}
	return buf.String()
}
