package functions

import (
	"testing"

	"github.com/spf13/afero"
)

func TestAddDocsToWorkspace(t *testing.T) {
	t.Run("domain__new_docs__adds_entry", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		if err := WriteWorkspaceConfig(fs, WorkspaceConfig{}); err != nil {
			t.Fatalf("WriteWorkspaceConfig: %v", err)
		}

		if err := AddDocsToWorkspace(fs, "handbook"); err != nil {
			t.Fatalf("AddDocsToWorkspace: %v", err)
		}

		config, err := ReadWorkspaceConfig(fs)
		if err != nil {
			t.Fatalf("ReadWorkspaceConfig: %v", err)
		}
		if len(config.Docs) != 1 || config.Docs[0].Name != "handbook" {
			t.Fatalf("expected docs handbook registered, got %+v", config.Docs)
		}
	})

	t.Run("boundary__existing_docs__no_duplicate", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		if err := WriteWorkspaceConfig(fs, WorkspaceConfig{
			Docs: []WorkspaceDocs{{Name: "runbook"}},
		}); err != nil {
			t.Fatalf("WriteWorkspaceConfig: %v", err)
		}

		if err := AddDocsToWorkspace(fs, "runbook"); err != nil {
			t.Fatalf("AddDocsToWorkspace: %v", err)
		}

		config, err := ReadWorkspaceConfig(fs)
		if err != nil {
			t.Fatalf("ReadWorkspaceConfig: %v", err)
		}
		if len(config.Docs) != 1 {
			t.Fatalf("expected no duplicate, got %d", len(config.Docs))
		}
	})
}

func TestRemoveDocsFromWorkspace(t *testing.T) {
	t.Run("domain__existing_docs__removes_entry", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		if err := WriteWorkspaceConfig(fs, WorkspaceConfig{
			Docs: []WorkspaceDocs{
				{Name: "alpha"},
				{Name: "beta"},
			},
		}); err != nil {
			t.Fatalf("WriteWorkspaceConfig: %v", err)
		}

		if err := RemoveDocsFromWorkspace(fs, "alpha"); err != nil {
			t.Fatalf("RemoveDocsFromWorkspace: %v", err)
		}

		config, err := ReadWorkspaceConfig(fs)
		if err != nil {
			t.Fatalf("ReadWorkspaceConfig: %v", err)
		}
		if len(config.Docs) != 1 || config.Docs[0].Name != "beta" {
			t.Fatalf("expected remaining docs beta, got %+v", config.Docs)
		}
	})
}
