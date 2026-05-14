package functions

import (
	"testing"

	"github.com/spf13/afero"
)

func TestAddInfraToWorkspace(t *testing.T) {
	t.Run("domain__new_infra__adds_entry", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		if err := WriteWorkspaceConfig(fs, WorkspaceConfig{}); err != nil {
			t.Fatalf("WriteWorkspaceConfig: %v", err)
		}

		if err := AddInfraToWorkspace(fs, "terraform-core"); err != nil {
			t.Fatalf("AddInfraToWorkspace: %v", err)
		}

		config, err := ReadWorkspaceConfig(fs)
		if err != nil {
			t.Fatalf("ReadWorkspaceConfig: %v", err)
		}
		if len(config.Infrastructure) != 1 || config.Infrastructure[0].Name != "terraform-core" {
			t.Fatalf("expected infra terraform-core registered, got %+v", config.Infrastructure)
		}
	})

	t.Run("boundary__existing_infra__no_duplicate", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		if err := WriteWorkspaceConfig(fs, WorkspaceConfig{
			Infrastructure: []WorkspaceInfra{{Name: "k8s-prod"}},
		}); err != nil {
			t.Fatalf("WriteWorkspaceConfig: %v", err)
		}

		if err := AddInfraToWorkspace(fs, "k8s-prod"); err != nil {
			t.Fatalf("AddInfraToWorkspace: %v", err)
		}

		config, err := ReadWorkspaceConfig(fs)
		if err != nil {
			t.Fatalf("ReadWorkspaceConfig: %v", err)
		}
		if len(config.Infrastructure) != 1 {
			t.Fatalf("expected no duplicate, got %d", len(config.Infrastructure))
		}
	})
}

func TestRemoveInfraFromWorkspace(t *testing.T) {
	t.Run("domain__existing_infra__removes_entry", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		if err := WriteWorkspaceConfig(fs, WorkspaceConfig{
			Infrastructure: []WorkspaceInfra{
				{Name: "alpha"},
				{Name: "beta"},
			},
		}); err != nil {
			t.Fatalf("WriteWorkspaceConfig: %v", err)
		}

		if err := RemoveInfraFromWorkspace(fs, "alpha"); err != nil {
			t.Fatalf("RemoveInfraFromWorkspace: %v", err)
		}

		config, err := ReadWorkspaceConfig(fs)
		if err != nil {
			t.Fatalf("ReadWorkspaceConfig: %v", err)
		}
		if len(config.Infrastructure) != 1 || config.Infrastructure[0].Name != "beta" {
			t.Fatalf("expected remaining infra beta, got %+v", config.Infrastructure)
		}
	})

	t.Run("boundary__missing_infra__noop", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		if err := WriteWorkspaceConfig(fs, WorkspaceConfig{
			Infrastructure: []WorkspaceInfra{{Name: "beta"}},
		}); err != nil {
			t.Fatalf("WriteWorkspaceConfig: %v", err)
		}

		if err := RemoveInfraFromWorkspace(fs, "missing"); err != nil {
			t.Fatalf("RemoveInfraFromWorkspace: %v", err)
		}

		config, err := ReadWorkspaceConfig(fs)
		if err != nil {
			t.Fatalf("ReadWorkspaceConfig: %v", err)
		}
		if len(config.Infrastructure) != 1 {
			t.Fatalf("expected entry preserved, got %d", len(config.Infrastructure))
		}
	})
}

func TestFindInfraIndex(t *testing.T) {
	t.Run("domain__match__returns_index", func(t *testing.T) {
		config := WorkspaceConfig{
			Infrastructure: []WorkspaceInfra{
				{Name: "alpha"},
				{Name: "beta"},
			},
		}
		if FindInfraIndex(config, "beta") != 1 {
			t.Fatalf("expected index 1")
		}
	})

	t.Run("complement__no_match__returns_negative", func(t *testing.T) {
		config := WorkspaceConfig{Infrastructure: []WorkspaceInfra{{Name: "alpha"}}}
		if FindInfraIndex(config, "missing") != -1 {
			t.Fatalf("expected -1")
		}
	})
}
