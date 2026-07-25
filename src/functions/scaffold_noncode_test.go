package functions

import (
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
)

func TestAddInfra(t *testing.T) {
	t.Run("domain__no_template__creates_empty_dir_and_registers", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		if err := WriteWorkspaceConfig(fs, WorkspaceConfig{}); err != nil {
			t.Fatalf("write workspace config: %v", err)
		}

		if err := AddInfra(fs, "terraform-core", "", nil); err != nil {
			t.Fatalf("AddInfra: %v", err)
		}

		infraPath := filepath.Join("repos", "infra", "terraform-core")
		info, err := fs.Stat(infraPath)
		if err != nil {
			t.Fatalf("expected infra dir: %v", err)
		}
		if !info.IsDir() {
			t.Fatalf("expected infra path to be a directory")
		}
		repoConfig, err := ReadRepoConfig(fs, infraPath)
		if err != nil {
			t.Fatalf("expected ocean.config.json: %v", err)
		}
		if repoConfig.Type != "infra" {
			t.Errorf("expected type=infra, got %q", repoConfig.Type)
		}

		config, err := ReadWorkspaceConfig(fs)
		if err != nil {
			t.Fatalf("ReadWorkspaceConfig: %v", err)
		}
		if len(config.Infrastructure) != 1 || config.Infrastructure[0].Name != "terraform-core" {
			t.Fatalf("expected infra registered: %+v", config.Infrastructure)
		}
	})

	t.Run("domain__with_template__substitutes_placeholders", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		if err := WriteWorkspaceConfig(fs, WorkspaceConfig{}); err != nil {
			t.Fatalf("write workspace config: %v", err)
		}
		templatePath := filepath.Join("repos", "templates", "terraform-base")
		if err := fs.MkdirAll(templatePath, 0o755); err != nil {
			t.Fatalf("setup template: %v", err)
		}
		if err := afero.WriteFile(fs, filepath.Join(templatePath, "main.tf"), []byte("region = \"{{ region }}\"\n"), 0o644); err != nil {
			t.Fatalf("write template file: %v", err)
		}

		if err := AddInfra(fs, "app-west", "terraform-base", map[string]string{"region": "us-west-2"}); err != nil {
			t.Fatalf("AddInfra: %v", err)
		}

		copied, err := afero.ReadFile(fs, filepath.Join("repos", "infra", "app-west", "main.tf"))
		if err != nil {
			t.Fatalf("read copied template: %v", err)
		}
		if string(copied) != "region = \"us-west-2\"\n" {
			t.Fatalf("placeholder not substituted: %q", string(copied))
		}
	})

	t.Run("boundary__already_exists__errors", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		if err := WriteWorkspaceConfig(fs, WorkspaceConfig{}); err != nil {
			t.Fatalf("write workspace config: %v", err)
		}
		if err := fs.MkdirAll(filepath.Join("repos", "infra", "existing"), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := AddInfra(fs, "existing", "", nil); err == nil {
			t.Fatalf("expected error when target exists")
		}
	})
}

func TestAddDocs(t *testing.T) {
	t.Run("domain__no_template__creates_empty_dir_and_registers", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		if err := WriteWorkspaceConfig(fs, WorkspaceConfig{}); err != nil {
			t.Fatalf("write workspace config: %v", err)
		}

		if err := AddDocs(fs, "handbook", "", nil); err != nil {
			t.Fatalf("AddDocs: %v", err)
		}

		docsPath := filepath.Join("repos", "docs", "handbook")
		if _, err := fs.Stat(docsPath); err != nil {
			t.Fatalf("expected docs dir: %v", err)
		}
		repoConfig, err := ReadRepoConfig(fs, docsPath)
		if err != nil {
			t.Fatalf("expected ocean.config.json: %v", err)
		}
		if repoConfig.Type != "docs" {
			t.Errorf("expected type=docs, got %q", repoConfig.Type)
		}

		config, err := ReadWorkspaceConfig(fs)
		if err != nil {
			t.Fatalf("ReadWorkspaceConfig: %v", err)
		}
		if len(config.Docs) != 1 || config.Docs[0].Name != "handbook" {
			t.Fatalf("expected docs registered: %+v", config.Docs)
		}
	})
}

func TestRemoveInfra(t *testing.T) {
	t.Run("domain__existing__removes_dir_and_entry", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		if err := WriteWorkspaceConfig(fs, WorkspaceConfig{
			Infrastructure: []WorkspaceInfra{{Name: "terraform-core"}},
		}); err != nil {
			t.Fatalf("write workspace config: %v", err)
		}
		if err := fs.MkdirAll(filepath.Join("repos", "infra", "terraform-core"), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}

		if err := RemoveInfra(fs, "terraform-core"); err != nil {
			t.Fatalf("RemoveInfra: %v", err)
		}

		if _, err := fs.Stat(filepath.Join("repos", "infra", "terraform-core")); err == nil {
			t.Errorf("expected directory removed")
		}
		config, err := ReadWorkspaceConfig(fs)
		if err != nil {
			t.Fatalf("ReadWorkspaceConfig: %v", err)
		}
		if len(config.Infrastructure) != 0 {
			t.Fatalf("expected entry removed: %+v", config.Infrastructure)
		}
	})
}
