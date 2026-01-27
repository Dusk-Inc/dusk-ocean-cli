package services

import (
	"testing"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/deps"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/workspace"
	"github.com/spf13/afero"
)

func TestNextServicePort(t *testing.T) {
	t.Run("domain__existing_services__returns_next_highest_port", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		config := workspace.WorkspaceConfig{
			Ports: workspace.WorkspacePorts{
				Allowed: workspace.WorkspacePortRange{
					Min: 4000,
					Max: 4999,
				},
			},
			Apps: []workspace.WorkspaceApp{
				{
					Name: "app",
					Services: []workspace.WorkspaceService{
						{Name: "svc1", Port: "4002"},
						{Name: "svc2", Port: "4005"},
					},
				},
			},
		}
		writeWorkspaceConfig(t, fs, config)

		port, err := NextServicePort(fs, "app")
		if err != nil {
			t.Fatalf("NextServicePort: %v", err)
		}
		if port != "4006" {
			t.Fatalf("expected port 4006, got %s", port)
		}
	})

	t.Run("boundary__no_allowed_ranges__defaults_to_3000", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		config := workspace.WorkspaceConfig{
			Apps: []workspace.WorkspaceApp{
				{
					Name:     "app",
					Services: []workspace.WorkspaceService{},
				},
			},
		}
		writeWorkspaceConfig(t, fs, config)

		port, err := NextServicePort(fs, "app")
		if err != nil {
			t.Fatalf("NextServicePort: %v", err)
		}
		if port != "3000" {
			t.Fatalf("expected port 3000, got %s", port)
		}
	})

	t.Run("complement__missing_app__returns_start_port", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		config := workspace.WorkspaceConfig{
			Ports: workspace.WorkspacePorts{
				Allowed: workspace.WorkspacePortRange{
					Min: 5000,
					Max: 5999,
				},
			},
			Apps: []workspace.WorkspaceApp{
				{
					Name: "other",
					Services: []workspace.WorkspaceService{
						{Name: "svc", Port: "5001"},
					},
				},
			},
		}
		writeWorkspaceConfig(t, fs, config)

		port, err := NextServicePort(fs, "app")
		if err != nil {
			t.Fatalf("NextServicePort: %v", err)
		}
		if port != "5000" {
			t.Fatalf("expected port 5000, got %s", port)
		}
	})

	t.Run("chaos__non_numeric_port__ignored", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		config := workspace.WorkspaceConfig{
			Ports: workspace.WorkspacePorts{
				Allowed: workspace.WorkspacePortRange{
					Min: 7000,
					Max: 7999,
				},
			},
			Apps: []workspace.WorkspaceApp{
				{
					Name: "app",
					Services: []workspace.WorkspaceService{
						{Name: "svc1", Port: "7002"},
						{Name: "svc2", Port: "not-a-number"},
					},
				},
			},
		}
		writeWorkspaceConfig(t, fs, config)

		port, err := NextServicePort(fs, "app")
		if err != nil {
			t.Fatalf("NextServicePort: %v", err)
		}
		if port != "7003" {
			t.Fatalf("expected port 7003, got %s", port)
		}
	})

	t.Run("domain__reserved_ports__skips_reserved", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		config := workspace.WorkspaceConfig{
			Ports: workspace.WorkspacePorts{
				Allowed: workspace.WorkspacePortRange{
					Min: 3000,
					Max: 3002,
				},
				Reserved: []workspace.WorkspaceReservedPort{
					{Name: "codex", Port: 3001},
				},
			},
			Apps: []workspace.WorkspaceApp{
				{
					Name: "app",
					Services: []workspace.WorkspaceService{
						{Name: "svc1", Port: "3000"},
					},
				},
			},
		}
		writeWorkspaceConfig(t, fs, config)

		port, err := NextServicePort(fs, "app")
		if err != nil {
			t.Fatalf("NextServicePort: %v", err)
		}
		if port != "3002" {
			t.Fatalf("expected port 3002, got %s", port)
		}
	})
}

func TestAddServiceToWorkspace(t *testing.T) {
	t.Run("domain__new_app__creates_app_and_service_with_defaults", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		writeWorkspaceConfig(t, fs, workspace.WorkspaceConfig{})

		err := AddServiceToWorkspace(fs, "{{app_name}}", "{{service_name}}", "3100", workspace.WorkspaceImage{}, "Dockerfile")
		if err != nil {
			t.Fatalf("AddServiceToWorkspace: %v", err)
		}

		config := readWorkspaceConfig(t, fs)
		if len(config.Apps) != 1 {
			t.Fatalf("expected 1 app, got %d", len(config.Apps))
		}
		app := config.Apps[0]
		if app.Name != "{{app_name}}" {
			t.Fatalf("expected app name {{app_name}}, got %s", app.Name)
		}
		if len(app.Services) != 1 {
			t.Fatalf("expected 1 service, got %d", len(app.Services))
		}
		service := app.Services[0]
		if service.Name != "{{service_name}}" {
			t.Fatalf("expected service name {{service_name}}, got %s", service.Name)
		}
		if service.Port != "3100" {
			t.Fatalf("expected port 3100, got %s", service.Port)
		}
		if service.Image.Name != "{{app_name}}__{{service_name}}" {
			t.Fatalf("expected image name {{app_name}}__{{service_name}}, got %s", service.Image.Name)
		}
		if service.Image.Tag != "dev" {
			t.Fatalf("expected image tag dev, got %s", service.Image.Tag)
		}
		if service.Dockerfile != "Dockerfile" {
			t.Fatalf("expected Dockerfile Dockerfile, got %s", service.Dockerfile)
		}
	})

	t.Run("boundary__existing_app_new_service__appends_service", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		config := workspace.WorkspaceConfig{
			Apps: []workspace.WorkspaceApp{
				{
					Name: "{{app_name}}",
					Services: []workspace.WorkspaceService{
						{
							Name:     "{{existing_service}}",
							Port:     "3000",
						},
					},
				},
			},
		}
		writeWorkspaceConfig(t, fs, config)

		image := workspace.WorkspaceImage{Name: "custom-image", Tag: "v1"}
		err := AddServiceToWorkspace(fs, "{{app_name}}", "{{service_name}}", "3001", image, "Dockerfile")
		if err != nil {
			t.Fatalf("AddServiceToWorkspace: %v", err)
		}

		config = readWorkspaceConfig(t, fs)
		app := config.Apps[0]
		if len(app.Services) != 2 {
			t.Fatalf("expected 2 services, got %d", len(app.Services))
		}
		service := findService(t, app, "{{service_name}}")
		if service.Port != "3001" {
			t.Fatalf("expected port 3001, got %s", service.Port)
		}
		if service.Image.Name != "custom-image" {
			t.Fatalf("expected image name custom-image, got %s", service.Image.Name)
		}
		if service.Image.Tag != "v1" {
			t.Fatalf("expected image tag v1, got %s", service.Image.Tag)
		}
		if service.Dockerfile != "Dockerfile" {
			t.Fatalf("expected Dockerfile Dockerfile, got %s", service.Dockerfile)
		}
	})

	t.Run("domain__existing_service__preserves_deps_and_dockerfile_when_blank", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		config := workspace.WorkspaceConfig{
			Apps: []workspace.WorkspaceApp{
				{
					Name: "app",
					Services: []workspace.WorkspaceService{
						{
							Name:       "api",
							Port:       "3000",
							Image:      workspace.WorkspaceImage{Name: "custom-image", Tag: "old"},
							Dockerfile: "Dockerfile.old",
							Deps:       []string{"db", "cache"},
						},
					},
				},
			},
		}
		writeWorkspaceConfig(t, fs, config)

		err := AddServiceToWorkspace(fs, "app", "api", "3005", workspace.WorkspaceImage{}, " ")
		if err != nil {
			t.Fatalf("AddServiceToWorkspace: %v", err)
		}

		config = readWorkspaceConfig(t, fs)
		service := findService(t, config.Apps[0], "api")
		if service.Port != "3005" {
			t.Fatalf("expected port 3005, got %s", service.Port)
		}
		if service.Dockerfile != "Dockerfile.old" {
			t.Fatalf("expected Dockerfile Dockerfile.old, got %s", service.Dockerfile)
		}
		if len(service.Deps) != 2 || service.Deps[0] != "db" || service.Deps[1] != "cache" {
			t.Fatalf("expected deps [db cache], got %v", service.Deps)
		}
		if service.Image.Name != "app__api" {
			t.Fatalf("expected image name app__api, got %s", service.Image.Name)
		}
		if service.Image.Tag != "dev" {
			t.Fatalf("expected image tag dev, got %s", service.Image.Tag)
		}
	})
}

func TestServiceImageReference(t *testing.T) {
	t.Run("domain__default_image_with_registry__returns_registry_reference", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		config := workspace.WorkspaceConfig{
			DockerRegistry: "ghcr.io/dusk-inc",
			Apps: []workspace.WorkspaceApp{
				{
					Name: "{{app_name}}",
					Services: []workspace.WorkspaceService{
						{
							Name:     "{{service_name}}",
						},
					},
				},
			},
		}
		writeWorkspaceConfig(t, fs, config)

		ref, err := ServiceImageReference(fs, "{{app_name}}", "{{service_name}}")
		if err != nil {
			t.Fatalf("ServiceImageReference: %v", err)
		}
		if ref != "ghcr.io/dusk-inc/{{app_name}}__{{service_name}}:dev" {
			t.Fatalf("expected registry reference, got %s", ref)
		}
	})

	t.Run("boundary__override_name__keeps_default_tag", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		config := workspace.WorkspaceConfig{
			DockerRegistry: "ghcr.io/dusk-inc",
			Apps: []workspace.WorkspaceApp{
				{
					Name: "{{app_name}}",
					Services: []workspace.WorkspaceService{
						{
							Name:     "{{service_name}}",
							Image: workspace.WorkspaceImage{
								Name: "custom-image",
							},
						},
					},
				},
			},
		}
		writeWorkspaceConfig(t, fs, config)

		ref, err := ServiceImageReference(fs, "{{app_name}}", "{{service_name}}")
		if err != nil {
			t.Fatalf("ServiceImageReference: %v", err)
		}
		if ref != "ghcr.io/dusk-inc/custom-image:dev" {
			t.Fatalf("expected registry reference, got %s", ref)
		}
	})

	t.Run("boundary__override_tag__keeps_default_name", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		config := workspace.WorkspaceConfig{
			DockerRegistry: "",
			Apps: []workspace.WorkspaceApp{
				{
					Name: "{{app_name}}",
					Services: []workspace.WorkspaceService{
						{
							Name:     "{{service_name}}",
							Image: workspace.WorkspaceImage{
								Tag: "v2",
							},
						},
					},
				},
			},
		}
		writeWorkspaceConfig(t, fs, config)

		ref, err := ServiceImageReference(fs, "{{app_name}}", "{{service_name}}")
		if err != nil {
			t.Fatalf("ServiceImageReference: %v", err)
		}
		if ref != "{{app_name}}__{{service_name}}:v2" {
			t.Fatalf("expected reference with custom tag, got %s", ref)
		}
	})

	t.Run("complement__missing_service__returns_default_reference", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		config := workspace.WorkspaceConfig{
			DockerRegistry: "ghcr.io/dusk-inc",
			Apps: []workspace.WorkspaceApp{
				{
					Name: "{{app_name}}",
					Services: []workspace.WorkspaceService{
						{
							Name:     "{{other_service}}",
						},
					},
				},
			},
		}
		writeWorkspaceConfig(t, fs, config)

		ref, err := ServiceImageReference(fs, "{{app_name}}", "{{service_name}}")
		if err != nil {
			t.Fatalf("ServiceImageReference: %v", err)
		}
		if ref != "ghcr.io/dusk-inc/{{app_name}}__{{service_name}}:dev" {
			t.Fatalf("expected default reference, got %s", ref)
		}
	})
}

func TestMakeServiceNode(t *testing.T) {
	t.Run("complement__missing_app__returns_error", func(t *testing.T) {
		config := workspace.WorkspaceConfig{
			Apps: []workspace.WorkspaceApp{
				{Name: "other"},
			},
		}

		_, err := MakeServiceNode(config, "app", "api")
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("complement__missing_service__returns_error", func(t *testing.T) {
		config := workspace.WorkspaceConfig{
			Apps: []workspace.WorkspaceApp{
				{
					Name: "app",
					Services: []workspace.WorkspaceService{
						{Name: "other"},
					},
				},
			},
		}

		_, err := MakeServiceNode(config, "app", "api")
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("domain__valid_service__returns_node", func(t *testing.T) {
		config := workspace.WorkspaceConfig{
			Apps: []workspace.WorkspaceApp{
				{
					Name: "app",
					Services: []workspace.WorkspaceService{
						{
							Name:     "api",
							Deps:     []string{"db"},
						},
					},
				},
			},
		}

		node, err := MakeServiceNode(config, "app", "api")
		if err != nil {
			t.Fatalf("MakeServiceNode: %v", err)
		}
		if node.Kind != deps.NodeService {
			t.Fatalf("expected node kind service, got %s", node.Kind)
		}
		if node.App != "app" {
			t.Fatalf("expected app app, got %s", node.App)
		}
		if node.Name != "api" {
			t.Fatalf("expected name api, got %s", node.Name)
		}
		if len(node.Deps) != 1 || node.Deps[0] != "db" {
			t.Fatalf("expected deps [db], got %v", node.Deps)
		}
	})
}

func TestRemoveServiceFromWorkspace(t *testing.T) {
	t.Run("domain__existing_service__removes_entry", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		writeWorkspaceConfig(t, fs, workspace.WorkspaceConfig{
			Apps: []workspace.WorkspaceApp{
				{
					Name: "alpha",
					Services: []workspace.WorkspaceService{
						{
							Name:     "api",
							Port:     "3000",
						},
						{
							Name:     "worker",
							Port:     "3001",
						},
					},
					Libraries: []workspace.WorkspaceLibrary{
						{
							Name:     "shared",
						},
					},
				},
			},
		})

		if err := RemoveServiceFromWorkspace(fs, "alpha", "api"); err != nil {
			t.Fatalf("RemoveServiceFromWorkspace: %v", err)
		}

		updated := readWorkspaceConfig(t, fs)
		if len(updated.Apps) != 1 {
			t.Fatalf("expected 1 app, got %d", len(updated.Apps))
		}
		if len(updated.Apps[0].Services) != 1 {
			t.Fatalf("expected 1 service, got %d", len(updated.Apps[0].Services))
		}
		if updated.Apps[0].Services[0].Name != "worker" {
			t.Fatalf("expected remaining service to be worker")
		}
		if len(updated.Apps[0].Libraries) != 1 || updated.Apps[0].Libraries[0].Name != "shared" {
			t.Fatalf("expected app libraries to remain")
		}
	})

	t.Run("boundary__missing_workspace_file__returns_error", func(t *testing.T) {
		fs := afero.NewMemMapFs()

		if err := RemoveServiceFromWorkspace(fs, "alpha", "api"); err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestFormatImageReference(t *testing.T) {
	t.Run("domain__name_tag_registry__formats_full_reference", func(t *testing.T) {
		ref := formatImageReference("ghcr.io/dusk-inc", workspace.WorkspaceImage{
			Name: "image",
			Tag:  "v1",
		})
		if ref != "ghcr.io/dusk-inc/image:v1" {
			t.Fatalf("expected ghcr.io/dusk-inc/image:v1, got %s", ref)
		}
	})

	t.Run("boundary__empty_registry__omits_registry_prefix", func(t *testing.T) {
		ref := formatImageReference("", workspace.WorkspaceImage{
			Name: "image",
			Tag:  "v1",
		})
		if ref != "image:v1" {
			t.Fatalf("expected image:v1, got %s", ref)
		}
	})

	t.Run("boundary__empty_tag__omits_tag", func(t *testing.T) {
		ref := formatImageReference("ghcr.io/dusk-inc", workspace.WorkspaceImage{
			Name: "image",
			Tag:  " ",
		})
		if ref != "ghcr.io/dusk-inc/image" {
			t.Fatalf("expected ghcr.io/dusk-inc/image, got %s", ref)
		}
	})

	t.Run("complement__empty_name__returns_empty", func(t *testing.T) {
		ref := formatImageReference("ghcr.io/dusk-inc", workspace.WorkspaceImage{
			Name: " ",
			Tag:  "v1",
		})
		if ref != "" {
			t.Fatalf("expected empty reference, got %s", ref)
		}
	})
}

func writeWorkspaceConfig(t *testing.T, fs afero.Fs, config workspace.WorkspaceConfig) {
	t.Helper()
	if err := workspace.WriteWorkspaceConfig(fs, config); err != nil {
		t.Fatalf("WriteWorkspaceConfig: %v", err)
	}
}

func readWorkspaceConfig(t *testing.T, fs afero.Fs) workspace.WorkspaceConfig {
	t.Helper()
	config, err := workspace.ReadWorkspaceConfig(fs)
	if err != nil {
		t.Fatalf("ReadWorkspaceConfig: %v", err)
	}
	return config
}

func findService(t *testing.T, app workspace.WorkspaceApp, name string) workspace.WorkspaceService {
	t.Helper()
	for _, service := range app.Services {
		if service.Name == name {
			return service
		}
	}
	t.Fatalf("service not found: %s", name)
	return workspace.WorkspaceService{}
}
