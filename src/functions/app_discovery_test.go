package functions

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
)

func seedAppWithSubRepos(t *testing.T, fs afero.Fs, appName string, services, libs, tests []string) {
	t.Helper()
	appRoot := filepath.Join("repos", "apps", appName)
	if err := fs.MkdirAll(appRoot, 0o755); err != nil {
		t.Fatalf("mkdir app: %v", err)
	}
	for _, svc := range services {
		dir := filepath.Join(appRoot, "services", svc)
		if err := fs.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir service: %v", err)
		}
		if err := afero.WriteFile(fs, filepath.Join(dir, "ocean.config.json"), []byte(`{"name":"`+svc+`","type":"service"}`), 0o644); err != nil {
			t.Fatalf("write service config: %v", err)
		}
	}
	for _, lib := range libs {
		dir := filepath.Join(appRoot, "libs", lib)
		if err := fs.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir lib: %v", err)
		}
		if err := afero.WriteFile(fs, filepath.Join(dir, "ocean.config.json"), []byte(`{"name":"`+lib+`","type":"library"}`), 0o644); err != nil {
			t.Fatalf("write lib config: %v", err)
		}
	}
	for _, test := range tests {
		dir := filepath.Join(appRoot, "testing", test)
		if err := fs.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir test: %v", err)
		}
		if err := afero.WriteFile(fs, filepath.Join(dir, "ocean.config.json"), []byte(`{"name":"`+test+`","type":"test"}`), 0o644); err != nil {
			t.Fatalf("write test config: %v", err)
		}
	}
}

func TestDiscoverAppSubRepos(t *testing.T) {
	t.Run("domain__finds_services_libs_and_tests", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		seedAppWithSubRepos(t, fs, "alpha",
			[]string{"api", "worker"},
			[]string{"shared"},
			[]string{"e2e"},
		)

		got, err := DiscoverAppSubRepos(fs, "alpha")
		if err != nil {
			t.Fatalf("DiscoverAppSubRepos: %v", err)
		}
		if len(got) != 4 {
			t.Fatalf("expected 4 sub-repos, got %d: %+v", len(got), got)
		}

		want := []DiscoveredAppSubRepo{
			{Kind: AppSubRepoKindService, Name: "api", Path: "repos/apps/alpha/services/api"},
			{Kind: AppSubRepoKindService, Name: "worker", Path: "repos/apps/alpha/services/worker"},
			{Kind: AppSubRepoKindLibrary, Name: "shared", Path: "repos/apps/alpha/libs/shared"},
			{Kind: AppSubRepoKindTest, Name: "e2e", Path: "repos/apps/alpha/testing/e2e"},
		}
		for i, w := range want {
			if got[i] != w {
				t.Errorf("entry %d: want %+v, got %+v", i, w, got[i])
			}
		}
	})

	t.Run("boundary__missing_subdirs__not_an_error", func(t *testing.T) {
		fs := afero.NewMemMapFs()

		if err := fs.MkdirAll("repos/apps/alpha", 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}

		got, err := DiscoverAppSubRepos(fs, "alpha")
		if err != nil {
			t.Fatalf("DiscoverAppSubRepos: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("expected zero discoveries, got %+v", got)
		}
	})

	t.Run("complement__directory_without_config__skipped", func(t *testing.T) {
		fs := afero.NewMemMapFs()

		if err := fs.MkdirAll("repos/apps/alpha/services/api", 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}

		got, err := DiscoverAppSubRepos(fs, "alpha")
		if err != nil {
			t.Fatalf("DiscoverAppSubRepos: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("expected zero discoveries (no config file), got %+v", got)
		}
	})

	t.Run("complement__only_one_level_deep", func(t *testing.T) {
		fs := afero.NewMemMapFs()

		nested := "repos/apps/alpha/services/group/nested"
		if err := fs.MkdirAll(nested, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := afero.WriteFile(fs, filepath.Join(nested, "ocean.config.json"), []byte("{}"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}

		got, err := DiscoverAppSubRepos(fs, "alpha")
		if err != nil {
			t.Fatalf("DiscoverAppSubRepos: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("expected zero discoveries (nesting too deep), got %+v", got)
		}
	})
}

func TestRegisterDiscoveredAppSubRepos(t *testing.T) {
	t.Run("domain__registers_every_unique_sub_repo", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		writeWorkspaceConfig(t, fs, WorkspaceConfig{})
		if err := AddProjectToWorkspace(fs, "placeholder"); err != nil {

			t.Fatalf("seed: %v", err)
		}
		if err := addAppToWorkspace(fs, "alpha"); err != nil {
			t.Fatalf("seed: %v", err)
		}
		seedAppWithSubRepos(t, fs, "alpha",
			[]string{"api", "worker"},
			[]string{"shared"},
			[]string{"e2e"},
		)

		var out bytes.Buffer
		if err := RegisterDiscoveredAppSubRepos(fs, &out, "alpha"); err != nil {
			t.Fatalf("RegisterDiscoveredAppSubRepos: %v", err)
		}

		cfg := readWorkspaceConfig(t, fs)
		appIdx := FindAppIndex(cfg, "alpha")
		if appIdx == -1 {
			t.Fatalf("alpha not in workspace")
		}
		if len(cfg.Apps[appIdx].Services) != 2 {
			t.Errorf("expected 2 services, got %d", len(cfg.Apps[appIdx].Services))
		}
		if len(cfg.Apps[appIdx].Libraries) != 1 {
			t.Errorf("expected 1 lib, got %d", len(cfg.Apps[appIdx].Libraries))
		}
		if len(cfg.Apps[appIdx].Testing) != 1 {
			t.Errorf("expected 1 test, got %d", len(cfg.Apps[appIdx].Testing))
		}

		for _, svc := range cfg.Apps[appIdx].Services {
			if svc.Remote != "" {
				t.Errorf("service %s should have no remote, got %q", svc.Name, svc.Remote)
			}
		}
		for _, lib := range cfg.Apps[appIdx].Libraries {
			if lib.Remote != "" {
				t.Errorf("library %s should have no remote, got %q", lib.Name, lib.Remote)
			}
		}
		for _, test := range cfg.Apps[appIdx].Testing {
			if test.Remote != "" {
				t.Errorf("test %s should have no remote, got %q", test.Name, test.Remote)
			}
		}
	})

	t.Run("boundary__skips_already_registered_and_logs", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		writeWorkspaceConfig(t, fs, WorkspaceConfig{})
		if err := addAppToWorkspace(fs, "alpha"); err != nil {
			t.Fatalf("seed: %v", err)
		}

		port, err := NextServicePort(fs, "alpha")
		if err != nil {
			t.Fatalf("port: %v", err)
		}
		if err := AddServiceToWorkspace(fs, "alpha", "api", port, DefaultServiceImage("alpha", "api"), "", ""); err != nil {
			t.Fatalf("seed service: %v", err)
		}
		seedAppWithSubRepos(t, fs, "alpha", []string{"api", "worker"}, nil, nil)

		var out bytes.Buffer
		if err := RegisterDiscoveredAppSubRepos(fs, &out, "alpha"); err != nil {
			t.Fatalf("RegisterDiscoveredAppSubRepos: %v", err)
		}

		log := out.String()
		if !strings.Contains(log, "discovered service alpha/api: already registered, skipping") {
			t.Errorf("expected skip log for api, got: %s", log)
		}
		if !strings.Contains(log, "discovered service alpha/worker: registered") {
			t.Errorf("expected register log for worker, got: %s", log)
		}

		cfg := readWorkspaceConfig(t, fs)
		appIdx := FindAppIndex(cfg, "alpha")
		if len(cfg.Apps[appIdx].Services) != 2 {
			t.Errorf("expected 2 services after run, got %d", len(cfg.Apps[appIdx].Services))
		}
	})

	t.Run("boundary__no_sub_repos__noop", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		writeWorkspaceConfig(t, fs, WorkspaceConfig{})
		if err := addAppToWorkspace(fs, "alpha"); err != nil {
			t.Fatalf("seed: %v", err)
		}

		if err := fs.MkdirAll("repos/apps/alpha", 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}

		var out bytes.Buffer
		if err := RegisterDiscoveredAppSubRepos(fs, &out, "alpha"); err != nil {
			t.Fatalf("RegisterDiscoveredAppSubRepos: %v", err)
		}
		if out.Len() != 0 {
			t.Errorf("expected no log output, got %q", out.String())
		}
	})
}

func TestRegisterRepo_AppDiscoversSubRepos(t *testing.T) {
	fs := afero.NewMemMapFs()
	seedScratchWorkspace(t, fs)
	seedAppWithSubRepos(t, fs, "alpha",
		[]string{"api"},
		[]string{"shared"},
		nil,
	)

	var out bytes.Buffer
	if err := RegisterRepo(fs, &out, "app", "alpha", "", "git@github.com:dusk-inc/alpha.git", ""); err != nil {
		t.Fatalf("RegisterRepo: %v", err)
	}

	cfg := readWorkspaceConfig(t, fs)
	appIdx := FindAppIndex(cfg, "alpha")
	if appIdx == -1 {
		t.Fatalf("alpha not in workspace")
	}
	if len(cfg.Apps[appIdx].Services) != 1 || cfg.Apps[appIdx].Services[0].Name != "api" {
		t.Errorf("expected service api, got %+v", cfg.Apps[appIdx].Services)
	}
	if len(cfg.Apps[appIdx].Libraries) != 1 || cfg.Apps[appIdx].Libraries[0].Name != "shared" {
		t.Errorf("expected library shared, got %+v", cfg.Apps[appIdx].Libraries)
	}

	log := out.String()
	if !strings.Contains(log, "discovered service alpha/api: registered") {
		t.Errorf("expected discovery log for api, got: %s", log)
	}
	if !strings.Contains(log, "discovered library alpha/shared: registered") {
		t.Errorf("expected discovery log for shared, got: %s", log)
	}
}
