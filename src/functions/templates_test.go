package functions

import (
	"bytes"
	"testing"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
	"github.com/spf13/afero"
)

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

	t.Run("domain__app_kind__appends_entry", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		writeWorkspaceConfig(t, fs, WorkspaceConfig{})

		if err := AddTemplateToWorkspace(fs, "base-app", tokens.TemplateKindApp); err != nil {
			t.Fatalf("AddTemplateToWorkspace: %v", err)
		}

		config := readWorkspaceConfig(t, fs)
		if len(config.Templates) != 1 {
			t.Fatalf("expected 1 template, got %d", len(config.Templates))
		}
		entry := config.Templates[0]
		if entry.Name != "base-app" || entry.Kind != tokens.TemplateKindApp {
			t.Fatalf("unexpected entry: %+v", entry)
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

func TestValidateTemplateKind(t *testing.T) {
	t.Run("domain__service_library_project_app_test_infra_docs__accepted", func(t *testing.T) {
		for _, kind := range []string{
			tokens.TemplateKindService,
			tokens.TemplateKindLibrary,
			tokens.TemplateKindProject,
			tokens.TemplateKindApp,
			tokens.TemplateKindTest,
			tokens.TemplateKindInfra,
			tokens.TemplateKindDocs,
		} {
			if err := ValidateTemplateKind(kind); err != nil {
				t.Errorf("expected %s to be accepted, got %v", kind, err)
			}
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

	repoConfig, err := ReadRepoConfig(fs, "repos/templates/ts-svc")
	if err != nil {
		t.Fatalf("ReadRepoConfig: %v", err)
	}
	if repoConfig.Type != tokens.TemplateKindService {
		t.Errorf("expected starter type=service, got %q", repoConfig.Type)
	}
}

func TestRegisterRepo_TemplateAppKind(t *testing.T) {
	fs := afero.NewMemMapFs()
	seedScratchWorkspace(t, fs)
	if err := fs.MkdirAll("repos/templates/base-app", 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := RegisterRepo(fs, &bytes.Buffer{}, tokens.RepoKindTemplate, "base-app", "", "", tokens.TemplateKindApp); err != nil {
		t.Fatalf("RegisterRepo: %v", err)
	}

	config := readWorkspaceConfig(t, fs)
	if len(config.Templates) != 1 || config.Templates[0].Name != "base-app" {
		t.Fatalf("expected template registered: %+v", config.Templates)
	}
	if config.Templates[0].Kind != tokens.TemplateKindApp {
		t.Errorf("expected kind=app, got %q", config.Templates[0].Kind)
	}
}

func TestFindTemplatesByKind(t *testing.T) {
	config := WorkspaceConfig{
		Templates: []WorkspaceTemplate{
			{Name: "ts-svc", Kind: tokens.TemplateKindService},
			{Name: "py-svc", Kind: tokens.TemplateKindService},
			{Name: "go-lib", Kind: tokens.TemplateKindLibrary},
			{Name: "base-app", Kind: tokens.TemplateKindApp},
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

	apps := FindTemplatesByKind(config, tokens.TemplateKindApp)
	if len(apps) != 1 || apps[0] != "base-app" {
		t.Fatalf("expected the app template to be returned, got %v", apps)
	}
}

func TestValidateAppTemplateDeps(t *testing.T) {
	t.Run("domain__no_deps__accepted", func(t *testing.T) {
		config := WorkspaceConfig{
			Templates: []WorkspaceTemplate{
				{Name: "base-app", Kind: tokens.TemplateKindApp, Deps: []WorkspaceDep{}},
			},
		}
		if err := ValidateAppTemplateDeps(config, "base-app"); err != nil {
			t.Fatalf("expected no-dep app template to pass, got %v", err)
		}
	})

	t.Run("boundary__unregistered_template__no_op", func(t *testing.T) {
		if err := ValidateAppTemplateDeps(WorkspaceConfig{}, "unregistered"); err != nil {
			t.Fatalf("expected no-op, got %v", err)
		}
	})

	t.Run("complement__declared_deps__rejected", func(t *testing.T) {
		config := WorkspaceConfig{
			Templates: []WorkspaceTemplate{
				{
					Name: "base-app",
					Kind: tokens.TemplateKindApp,
					Deps: []WorkspaceDep{{Lib: "shared", From: "global"}},
				},
			},
		}
		if err := ValidateAppTemplateDeps(config, "base-app"); err == nil {
			t.Fatalf("expected an app template with deps to be rejected")
		}
	})
}

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

func TestValidateTemplateDepsForTarget_FilesystemOnlyTemplateNoOp(t *testing.T) {
	config := WorkspaceConfig{}
	target := Target{Kind: TargetService, App: "alpha", Name: "api"}
	if err := ValidateTemplateDepsForTarget("/tmp", config, "unregistered", target); err != nil {
		t.Fatalf("expected no-op, got %v", err)
	}
}

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
