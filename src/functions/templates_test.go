package functions

import (
	"bytes"
	"testing"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
	"github.com/spf13/afero"
)

// TestAddTemplateToWorkspace covers REQ 19.1/19.2: registering a template
// drops a WorkspaceTemplate entry into ocean.workspace.json with the
// requested kind. App kind is rejected up-front.
func TestAddTemplateToWorkspace(t *testing.T) {
	t.Run("domain__service_kind__appends_entry", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		writeWorkspaceConfig(t, fs, WorkspaceConfig{})

		if err := AddTemplateToWorkspace(fs, "ts-svc", tokens.TemplateKindService); err != nil {
			t.Fatalf("AddTemplateToWorkspace: %v", err)
		}

		config := readWorkspaceConfig(t, fs)
		if len(config.Templates) != 1 {
			t.Fatalf("expected 1 template, got %d", len(config.Templates))
		}
		entry := config.Templates[0]
		if entry.Name != "ts-svc" || entry.Kind != tokens.TemplateKindService {
			t.Fatalf("unexpected entry: %+v", entry)
		}
		if entry.Deps == nil {
			t.Fatalf("expected initialized Deps slice")
		}
	})

	t.Run("complement__app_kind__rejected", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		writeWorkspaceConfig(t, fs, WorkspaceConfig{})

		if err := AddTemplateToWorkspace(fs, "tpl", tokens.RepoKindApp); err == nil {
			t.Fatalf("expected app template kind to be rejected")
		}
	})

	t.Run("complement__duplicate_name__rejected", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		writeWorkspaceConfig(t, fs, WorkspaceConfig{
			Templates: []WorkspaceTemplate{
				{Name: "tpl", Kind: tokens.TemplateKindService, Deps: []WorkspaceDep{}},
			},
		})

		if err := AddTemplateToWorkspace(fs, "tpl", tokens.TemplateKindService); err == nil {
			t.Fatalf("expected duplicate template to be rejected")
		}
	})
}

// TestValidateTemplateKind covers REQ 19.3/19.4/19.5: only the three
// allowed kinds are accepted, app is explicitly rejected, and missing
// values yield a clear error.
func TestValidateTemplateKind(t *testing.T) {
	t.Run("domain__service_library_project__accepted", func(t *testing.T) {
		for _, kind := range []string{tokens.TemplateKindService, tokens.TemplateKindLibrary, tokens.TemplateKindProject} {
			if err := ValidateTemplateKind(kind); err != nil {
				t.Errorf("expected %s to be accepted, got %v", kind, err)
			}
		}
	})

	t.Run("complement__app__explicitly_rejected", func(t *testing.T) {
		err := ValidateTemplateKind(tokens.RepoKindApp)
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("complement__empty__required", func(t *testing.T) {
		if err := ValidateTemplateKind(""); err == nil {
			t.Fatalf("expected required error")
		}
	})

	t.Run("complement__unknown__rejected", func(t *testing.T) {
		if err := ValidateTemplateKind("widget"); err == nil {
			t.Fatalf("expected error")
		}
	})
}

// TestValidateTemplateKindFlag covers REQ 19.5: --template-kind only
// applies when --kind is template, and is required when it is.
func TestValidateTemplateKindFlag(t *testing.T) {
	if err := ValidateTemplateKindFlag(tokens.RepoKindProject, ""); err != nil {
		t.Errorf("non-template kind without template-kind should pass, got %v", err)
	}
	if err := ValidateTemplateKindFlag(tokens.RepoKindProject, tokens.TemplateKindService); err == nil {
		t.Errorf("template-kind with non-template kind should fail")
	}
	if err := ValidateTemplateKindFlag(tokens.RepoKindTemplate, ""); err == nil {
		t.Errorf("template kind without template-kind should fail")
	}
	if err := ValidateTemplateKindFlag(tokens.RepoKindTemplate, tokens.TemplateKindService); err != nil {
		t.Errorf("template kind with valid template-kind should pass, got %v", err)
	}
}

// TestRegisterRepo_Template covers REQ 19.1/19.2: registering a template
// writes a starter ocean.config.json whose `type` field carries the
// template kind, and adds a WorkspaceTemplate entry to workspace config.
func TestRegisterRepo_Template(t *testing.T) {
	fs := afero.NewMemMapFs()
	seedScratchWorkspace(t, fs)
	if err := fs.MkdirAll("repos/templates/ts-svc", 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := RegisterRepo(fs, &bytes.Buffer{}, tokens.RepoKindTemplate, "ts-svc", "", "", tokens.TemplateKindService); err != nil {
		t.Fatalf("RegisterRepo: %v", err)
	}

	config := readWorkspaceConfig(t, fs)
	if len(config.Templates) != 1 || config.Templates[0].Name != "ts-svc" {
		t.Fatalf("expected template registered: %+v", config.Templates)
	}
	if config.Templates[0].Kind != tokens.TemplateKindService {
		t.Errorf("expected kind=service, got %q", config.Templates[0].Kind)
	}
	if config.Templates[0].Remote != tokens.RemoteNone {
		t.Errorf("expected RemoteNone sentinel, got %q", config.Templates[0].Remote)
	}

	// Starter ocean.config.json should declare the template's downstream type.
	repoConfig, err := ReadRepoConfig(fs, "repos/templates/ts-svc")
	if err != nil {
		t.Fatalf("ReadRepoConfig: %v", err)
	}
	if repoConfig.Type != tokens.TemplateKindService {
		t.Errorf("expected starter type=service, got %q", repoConfig.Type)
	}
}

// TestRegisterRepo_TemplateAppKindRejected covers REQ 19.3.
func TestRegisterRepo_TemplateAppKindRejected(t *testing.T) {
	fs := afero.NewMemMapFs()
	seedScratchWorkspace(t, fs)
	if err := fs.MkdirAll("repos/templates/bad", 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	err := RegisterRepo(fs, &bytes.Buffer{}, tokens.RepoKindTemplate, "bad", "", "", tokens.RepoKindApp)
	if err == nil {
		t.Fatalf("expected --template-kind app to be rejected")
	}
}

// TestFindTemplatesByKind covers REQ 19.6: workspace-registered templates
// are filterable by their kind, and "app" returns nothing even if a stale
// entry happens to exist.
func TestFindTemplatesByKind(t *testing.T) {
	config := WorkspaceConfig{
		Templates: []WorkspaceTemplate{
			{Name: "ts-svc", Kind: tokens.TemplateKindService},
			{Name: "py-svc", Kind: tokens.TemplateKindService},
			{Name: "go-lib", Kind: tokens.TemplateKindLibrary},
		},
	}

	svcs := FindTemplatesByKind(config, tokens.TemplateKindService)
	if len(svcs) != 2 {
		t.Fatalf("expected 2 service templates, got %v", svcs)
	}

	libs := FindTemplatesByKind(config, tokens.TemplateKindLibrary)
	if len(libs) != 1 {
		t.Fatalf("expected 1 library template, got %v", libs)
	}

	apps := FindTemplatesByKind(config, tokens.RepoKindApp)
	if len(apps) != 0 {
		t.Fatalf("expected zero app templates (apps not template-able), got %v", apps)
	}
}

// TestValidateTemplateDepsForTarget_RejectsMissingDep covers REQ 19.7:
// pre-validation must reject before any files are copied when a template
// dep references a library that doesn't exist.
func TestValidateTemplateDepsForTarget_RejectsMissingDep(t *testing.T) {
	config := WorkspaceConfig{
		Templates: []WorkspaceTemplate{
			{
				Name: "ts-svc",
				Kind: tokens.TemplateKindService,
				Deps: []WorkspaceDep{
					{Lib: "ghost", From: "global"},
				},
			},
		},
	}

	target := Target{
		Kind: TargetService,
		App:  "alpha",
		Name: "api",
		Path: "/tmp/repos/apps/alpha/services/api",
	}

	err := ValidateTemplateDepsForTarget("/tmp", config, "ts-svc", target)
	if err == nil {
		t.Fatalf("expected error for missing dep")
	}
}

// TestValidateTemplateDepsForTarget_AcceptsValidDep covers the happy path
// for REQ 19.7: a template dep that resolves to a real global library is
// accepted, even though the target itself is not yet in workspace config.
func TestValidateTemplateDepsForTarget_AcceptsValidDep(t *testing.T) {
	config := WorkspaceConfig{
		Libraries: []WorkspaceLibrary{
			{Name: "shared", Deps: []WorkspaceDep{}},
		},
		Templates: []WorkspaceTemplate{
			{
				Name: "ts-svc",
				Kind: tokens.TemplateKindService,
				Deps: []WorkspaceDep{
					{Lib: "shared", From: "global"},
				},
			},
		},
	}

	target := Target{
		Kind: TargetService,
		App:  "alpha",
		Name: "api",
		Path: "/tmp/repos/apps/alpha/services/api",
	}

	if err := ValidateTemplateDepsForTarget("/tmp", config, "ts-svc", target); err != nil {
		t.Fatalf("expected validation to pass, got %v", err)
	}
}

// TestValidateTemplateDepsForTarget_FilesystemOnlyTemplateNoOp covers a
// safety property of REQ 19.7: a template that is NOT registered in
// workspace config has no declared deps, so pre-validation is a no-op.
func TestValidateTemplateDepsForTarget_FilesystemOnlyTemplateNoOp(t *testing.T) {
	config := WorkspaceConfig{}
	target := Target{Kind: TargetService, App: "alpha", Name: "api"}
	if err := ValidateTemplateDepsForTarget("/tmp", config, "unregistered", target); err != nil {
		t.Fatalf("expected no-op, got %v", err)
	}
}

// TestCollectWorkspaceNodes_ExcludesTemplates covers REQ 19.9: templates
// must never appear in the dependency graph that build/check/refresh use.
func TestCollectWorkspaceNodes_ExcludesTemplates(t *testing.T) {
	config := WorkspaceConfig{
		Libraries: []WorkspaceLibrary{
			{Name: "real-lib", Deps: []WorkspaceDep{}},
		},
		Templates: []WorkspaceTemplate{
			{Name: "tpl", Kind: tokens.TemplateKindService, Deps: []WorkspaceDep{}},
		},
	}
	nodes := CollectWorkspaceNodes(config)
	for _, node := range nodes {
		if node.Name == "tpl" {
			t.Fatalf("template leaked into dependency graph: %+v", node)
		}
	}
	if len(nodes) != 1 || nodes[0].Name != "real-lib" {
		t.Fatalf("expected only real-lib in graph, got %+v", nodes)
	}
}
