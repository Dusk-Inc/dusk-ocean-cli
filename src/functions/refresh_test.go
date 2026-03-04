package functions

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

func makeTestCmd(out *bytes.Buffer) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	return cmd
}

func writeOceanConfig(t *testing.T, fs afero.Fs, repoPath string, content string) {
	t.Helper()
	if err := fs.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", repoPath, err)
	}
	if err := afero.WriteFile(fs, repoPath+"/ocean.json", []byte(content), 0o644); err != nil {
		t.Fatalf("write ocean.json: %v", err)
	}
}

func TestRunInstall(t *testing.T) {
	t.Run("domain__no_install_task__skips_with_message", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		repoPath := "/repo/my-lib"
		writeOceanConfig(t, fs, repoPath, `{"name":"my-lib","language":"go","type":"library","tasks":{}}`)

		var out bytes.Buffer
		cmd := makeTestCmd(&out)

		err := runInstall(cmd, fs, "global library my-lib", repoPath)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !strings.Contains(out.String(), "install skipped") {
			t.Fatalf("expected skip message, got: %q", out.String())
		}
	})

	t.Run("boundary__empty_install_command__skips_with_message", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		repoPath := "/repo/my-lib"
		writeOceanConfig(t, fs, repoPath, `{"name":"my-lib","language":"go","type":"library","tasks":{"install":""}}`)

		var out bytes.Buffer
		cmd := makeTestCmd(&out)

		err := runInstall(cmd, fs, "global library my-lib", repoPath)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !strings.Contains(out.String(), "install skipped") {
			t.Fatalf("expected skip message, got: %q", out.String())
		}
	})
}
