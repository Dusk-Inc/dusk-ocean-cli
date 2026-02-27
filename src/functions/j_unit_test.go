package functions

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/afero"
)

func TestMakeJUnitReport(t *testing.T) {
	t.Run("domain__success_no_error__writes_testsuite", func(t *testing.T) {
		payload, err := makeJUnitReport("Check Alpha", []byte("stdout"), []byte("stderr"), nil)
		if err != nil {
			t.Fatalf("makeJUnitReport: %v", err)
		}

		output := string(payload)
		if !strings.Contains(output, `tests="1" failures="0"`) {
			t.Fatalf("expected testsuite attributes, got %s", output)
		}
		if strings.Contains(output, "<failure") {
			t.Fatalf("expected no failure, got %s", output)
		}
		if !strings.Contains(output, "<system-out>stdout</system-out>") {
			t.Fatalf("expected stdout, got %s", output)
		}
		if !strings.Contains(output, "<system-err>stderr</system-err>") {
			t.Fatalf("expected stderr, got %s", output)
		}
	})

	t.Run("boundary__empty_label__writes_empty_name", func(t *testing.T) {
		payload, err := makeJUnitReport("", nil, nil, nil)
		if err != nil {
			t.Fatalf("makeJUnitReport: %v", err)
		}

		output := string(payload)
		if !strings.Contains(output, `name=""`) {
			t.Fatalf("expected empty name, got %s", output)
		}
	})

	t.Run("complement__exec_error_with_stderr__includes_failure", func(t *testing.T) {
		payload, err := makeJUnitReport("Check Fail", []byte("out"), []byte("boom"), errTest("fail"))
		if err != nil {
			t.Fatalf("makeJUnitReport: %v", err)
		}

		output := string(payload)
		if !strings.Contains(output, `failures="1"`) {
			t.Fatalf("expected failures=1, got %s", output)
		}
		if !strings.Contains(output, `<failure message="fail">boom</failure>`) {
			t.Fatalf("expected failure content, got %s", output)
		}
	})

	t.Run("chaos__escape_values__escapes_xml", func(t *testing.T) {
		payload, err := makeJUnitReport("a & b", []byte("<out>"), []byte("x & y"), nil)
		if err != nil {
			t.Fatalf("makeJUnitReport: %v", err)
		}

		output := string(payload)
		if !strings.Contains(output, `name="a &amp; b"`) {
			t.Fatalf("expected escaped label, got %s", output)
		}
		if !strings.Contains(output, "<system-out>&lt;out&gt;</system-out>") {
			t.Fatalf("expected escaped stdout, got %s", output)
		}
		if !strings.Contains(output, "<system-err>x &amp; y</system-err>") {
			t.Fatalf("expected escaped stderr, got %s", output)
		}
	})
}

func TestFindJUnitReport(t *testing.T) {
	t.Run("domain__latest_junit_xml__returns_latest_path", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/workspace"
		targetPath := filepath.Join(root, "repos", "apps", "alpha")
		if err := fs.MkdirAll(targetPath, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}

		startedAt := time.Now().Add(-time.Minute)
		firstPath := filepath.Join(targetPath, "report-1.xml")
		writeFile(t, fs, firstPath, `<testsuite name="a"></testsuite>`, startedAt.Add(10*time.Second))
		secondPath := filepath.Join(root, ".ocean", "results", "report-2.xml")
		writeFile(t, fs, secondPath, `<testsuites></testsuites>`, startedAt.Add(20*time.Second))
		ignoredPath := filepath.Join(targetPath, "ignored.xml")
		writeFile(t, fs, ignoredPath, `<root></root>`, startedAt.Add(30*time.Second))

		selected, err := findJUnitReport(fs, root, targetPath, startedAt)
		if err != nil {
			t.Fatalf("findJUnitReport: %v", err)
		}
		if selected != secondPath {
			t.Fatalf("expected %s, got %s", secondPath, selected)
		}
	})

	t.Run("boundary__no_recent_junit_xml__returns_empty", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/workspace"
		targetPath := filepath.Join(root, "repos", "apps", "beta")
		if err := fs.MkdirAll(targetPath, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}

		startedAt := time.Now()
		oldPath := filepath.Join(targetPath, "old.xml")
		writeFile(t, fs, oldPath, `<testsuite name="a"></testsuite>`, startedAt.Add(-time.Minute))

		selected, err := findJUnitReport(fs, root, targetPath, startedAt)
		if err != nil {
			t.Fatalf("findJUnitReport: %v", err)
		}
		if selected != "" {
			t.Fatalf("expected empty, got %s", selected)
		}
	})

	t.Run("chaos__target_is_file__returns_empty", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/workspace"
		targetPath := filepath.Join(root, "repos", "apps", "gamma")
		if err := fs.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		writeFile(t, fs, targetPath, "not-a-dir", time.Now())

		selected, err := findJUnitReport(fs, root, targetPath, time.Now().Add(-time.Minute))
		if err != nil {
			t.Fatalf("findJUnitReport: %v", err)
		}
		if selected != "" {
			t.Fatalf("expected empty, got %s", selected)
		}
	})
}

func TestWriteCheckResults(t *testing.T) {
	t.Run("domain__existing_report__copies_to_results_dir", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/workspace"
		targetPath := filepath.Join(root, "repos", "apps", "delta")
		if err := fs.MkdirAll(targetPath, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}

		startedAt := time.Now().Add(-time.Minute)
		reportPath := filepath.Join(targetPath, "report.xml")
		writeFile(t, fs, reportPath, `<testsuite name="a"></testsuite>`, startedAt.Add(10*time.Second))

		if err := WriteCheckResults(fs, root, targetPath, "Check Delta", startedAt, []byte("out"), []byte("err"), nil); err != nil {
			t.Fatalf("WriteCheckResults: %v", err)
		}

		resultsDir, err := makeResultsDir(root, targetPath)
		if err != nil {
			t.Fatalf("makeResultsDir: %v", err)
		}
		destPath := filepath.Join(resultsDir, filepath.Base(reportPath))
		payload, err := afero.ReadFile(fs, destPath)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		if string(payload) != `<testsuite name="a"></testsuite>` {
			t.Fatalf("expected copied report, got %s", string(payload))
		}
	})

	t.Run("boundary__no_report_found__writes_generated_file", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		root := "/workspace"
		targetPath := filepath.Join(root, "repos", "apps", "epsilon")
		if err := fs.MkdirAll(targetPath, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}

		startedAt := time.Now().Add(-time.Minute)
		if err := WriteCheckResults(fs, root, targetPath, "My Check", startedAt, []byte("ok"), nil, nil); err != nil {
			t.Fatalf("WriteCheckResults: %v", err)
		}

		resultsDir, err := makeResultsDir(root, targetPath)
		if err != nil {
			t.Fatalf("makeResultsDir: %v", err)
		}
		filePath := filepath.Join(resultsDir, fmtCheckResultFileName("My Check"))
		payload, err := afero.ReadFile(fs, filePath)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		if !strings.Contains(string(payload), "<testsuite") {
			t.Fatalf("expected junit report, got %s", string(payload))
		}
	})
}

type errTest string

func (e errTest) Error() string {
	return string(e)
}

func writeFile(t *testing.T, fs afero.Fs, path string, content string, modTime time.Time) {
	t.Helper()
	if err := fs.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := afero.WriteFile(fs, path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := fs.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
}
