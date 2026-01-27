package compose

import (
	"testing"

	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

func makeServiceDef(image string, ports any) map[string]any {
	service := map[string]any{}
	if image != "" {
		service["image"] = image
	}
	if ports != nil {
		service["ports"] = ports
	}
	return service
}

func makeComposeRoot(services map[string]any) map[string]any {
	root := map[string]any{}
	if services != nil {
		root["services"] = services
	}
	return root
}

func writeYAML(fs afero.Fs, path string, value any) error {
	payload, err := yaml.Marshal(value)
	if err != nil {
		return err
	}
	return afero.WriteFile(fs, path, payload, 0o644)
}

func TestReadComposeRoot(t *testing.T) {
	t.Run("domain__valid_yaml__returns_root", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		path := "/compose.yaml"
		root := makeComposeRoot(map[string]any{
			"web": makeServiceDef("nginx:latest", []string{"8080:80"}),
		})

		if err := writeYAML(fs, path, root); err != nil {
			t.Fatalf("writeYAML: %v", err)
		}

		result, err := ReadComposeRoot(fs, path)
		if err != nil {
			t.Fatalf("ReadComposeRoot: %v", err)
		}
		if _, ok := result["services"]; !ok {
			t.Fatalf("expected services key")
		}
	})

	t.Run("boundary__empty_file__returns_empty_map", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		path := "/compose.yaml"

		if err := afero.WriteFile(fs, path, []byte{}, 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		result, err := ReadComposeRoot(fs, path)
		if err != nil {
			t.Fatalf("ReadComposeRoot: %v", err)
		}
		if len(result) != 0 {
			t.Fatalf("expected empty map")
		}
	})

	t.Run("complement__missing_file__returns_empty_map", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		path := "/compose.yaml"

		result, err := ReadComposeRoot(fs, path)
		if err != nil {
			t.Fatalf("ReadComposeRoot: %v", err)
		}
		if len(result) != 0 {
			t.Fatalf("expected empty map")
		}
	})

	t.Run("complement__invalid_yaml__returns_error", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		path := "/compose.yaml"

		if err := afero.WriteFile(fs, path, []byte("services: ["), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		if _, err := ReadComposeRoot(fs, path); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("boundary__null_yaml__returns_empty_map", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		path := "/compose.yaml"

		if err := afero.WriteFile(fs, path, []byte("null\n"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		result, err := ReadComposeRoot(fs, path)
		if err != nil {
			t.Fatalf("ReadComposeRoot: %v", err)
		}
		if len(result) != 0 {
			t.Fatalf("expected empty map")
		}
	})
}

func TestReadComposeServices(t *testing.T) {
	t.Run("domain__services_present__returns_map", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		path := "/compose.yaml"
		root := makeComposeRoot(map[string]any{
			"api": makeServiceDef("golang:1.22", []string{"8080:8080"}),
		})

		if err := writeYAML(fs, path, root); err != nil {
			t.Fatalf("writeYAML: %v", err)
		}

		services, err := ReadComposeServices(fs, path)
		if err != nil {
			t.Fatalf("ReadComposeServices: %v", err)
		}
		if _, ok := services["api"]; !ok {
			t.Fatalf("expected api service")
		}
	})

	t.Run("complement__missing_services__returns_empty_map", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		path := "/compose.yaml"
		root := map[string]any{
			"version": "3",
		}

		if err := writeYAML(fs, path, root); err != nil {
			t.Fatalf("writeYAML: %v", err)
		}

		services, err := ReadComposeServices(fs, path)
		if err != nil {
			t.Fatalf("ReadComposeServices: %v", err)
		}
		if len(services) != 0 {
			t.Fatalf("expected empty map")
		}
	})

	t.Run("complement__invalid_services_type__returns_error", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		path := "/compose.yaml"
		root := map[string]any{
			"services": 123,
		}

		if err := writeYAML(fs, path, root); err != nil {
			t.Fatalf("writeYAML: %v", err)
		}

		if _, err := ReadComposeServices(fs, path); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("complement__invalid_service_definition__returns_error", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		path := "/compose.yaml"
		root := makeComposeRoot(map[string]any{
			"web": "invalid",
		})

		if err := writeYAML(fs, path, root); err != nil {
			t.Fatalf("writeYAML: %v", err)
		}

		if _, err := ReadComposeServices(fs, path); err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestReadComposeService(t *testing.T) {
	t.Run("domain__service_exists__returns_service", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		path := "/compose.yaml"
		root := makeComposeRoot(map[string]any{
			"worker": makeServiceDef("alpine:latest", nil),
		})

		if err := writeYAML(fs, path, root); err != nil {
			t.Fatalf("writeYAML: %v", err)
		}

		service, err := ReadComposeService(fs, path, "worker")
		if err != nil {
			t.Fatalf("ReadComposeService: %v", err)
		}
		if ReadServiceImage(service) != "alpine:latest" {
			t.Fatalf("expected image")
		}
	})

	t.Run("complement__missing_services__returns_error", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		path := "/compose.yaml"
		root := map[string]any{
			"version": "3",
		}

		if err := writeYAML(fs, path, root); err != nil {
			t.Fatalf("writeYAML: %v", err)
		}

		if _, err := ReadComposeService(fs, path, "worker"); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("complement__invalid_services_type__returns_error", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		path := "/compose.yaml"
		root := map[string]any{
			"services": "invalid",
		}

		if err := writeYAML(fs, path, root); err != nil {
			t.Fatalf("writeYAML: %v", err)
		}

		if _, err := ReadComposeService(fs, path, "worker"); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("complement__missing_service__returns_error", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		path := "/compose.yaml"
		root := makeComposeRoot(map[string]any{
			"api": makeServiceDef("golang:1.22", nil),
		})

		if err := writeYAML(fs, path, root); err != nil {
			t.Fatalf("writeYAML: %v", err)
		}

		if _, err := ReadComposeService(fs, path, "worker"); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("complement__invalid_service_type__returns_error", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		path := "/compose.yaml"
		root := makeComposeRoot(map[string]any{
			"worker": "invalid",
		})

		if err := writeYAML(fs, path, root); err != nil {
			t.Fatalf("writeYAML: %v", err)
		}

		if _, err := ReadComposeService(fs, path, "worker"); err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestReadServiceImage(t *testing.T) {
	t.Run("domain__string_value__returns_trimmed", func(t *testing.T) {
		service := makeServiceDef("  nginx:latest  ", nil)

		image := ReadServiceImage(service)
		if image != "nginx:latest" {
			t.Fatalf("expected trimmed image")
		}
	})

	t.Run("boundary__missing_key__returns_empty_string", func(t *testing.T) {
		service := map[string]any{}

		image := ReadServiceImage(service)
		if image != "" {
			t.Fatalf("expected empty string")
		}
	})

	t.Run("chaos__non_string_value__returns_empty_string", func(t *testing.T) {
		service := map[string]any{
			"image": 123,
		}

		image := ReadServiceImage(service)
		if image != "" {
			t.Fatalf("expected empty string")
		}
	})
}

func TestReadServicePorts(t *testing.T) {
	t.Run("domain__string_slice__returns_copy", func(t *testing.T) {
		ports := []string{"8080:80"}
		service := makeServiceDef("", ports)

		result := ReadServicePorts(service)
		if len(result) != 1 || result[0] != "8080:80" {
			t.Fatalf("expected ports")
		}
		result[0] = "changed"
		if ports[0] != "8080:80" {
			t.Fatalf("expected copy")
		}
	})

	t.Run("boundary__single_string__returns_slice", func(t *testing.T) {
		service := makeServiceDef("", "8080:80")

		result := ReadServicePorts(service)
		if len(result) != 1 || result[0] != "8080:80" {
			t.Fatalf("expected ports")
		}
	})

	t.Run("complement__missing_ports__returns_nil", func(t *testing.T) {
		service := map[string]any{}

		result := ReadServicePorts(service)
		if result != nil {
			t.Fatalf("expected nil")
		}
	})

	t.Run("chaos__mixed_slice__returns_strings_only", func(t *testing.T) {
		service := makeServiceDef("", []any{"8080:80", 123})

		result := ReadServicePorts(service)
		if len(result) != 1 || result[0] != "8080:80" {
			t.Fatalf("expected string-only ports")
		}
	})

	t.Run("complement__unsupported_type__returns_nil", func(t *testing.T) {
		service := makeServiceDef("", map[string]any{"port": "8080:80"})

		result := ReadServicePorts(service)
		if result != nil {
			t.Fatalf("expected nil")
		}
	})
}

func TestReadHostPort(t *testing.T) {
	t.Run("domain__simple_mapping__returns_host", func(t *testing.T) {
		value := ReadHostPort("8080:80")
		if value != "8080" {
			t.Fatalf("expected host port")
		}
	})

	t.Run("boundary__missing_separator__returns_empty", func(t *testing.T) {
		value := ReadHostPort("80")
		if value != "" {
			t.Fatalf("expected empty")
		}
	})

	t.Run("domain__ip_host_container__returns_host", func(t *testing.T) {
		value := ReadHostPort("127.0.0.1:8080:80")
		if value != "8080" {
			t.Fatalf("expected host port")
		}
	})

	t.Run("boundary__proto_suffix__returns_host", func(t *testing.T) {
		value := ReadHostPort("127.0.0.1:8080:80/udp")
		if value != "8080" {
			t.Fatalf("expected host port")
		}
	})
}

func TestAddServiceToCompose(t *testing.T) {
	t.Run("domain__new_service__writes_compose", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		path := "/compose.yaml"
		service := makeServiceDef("nginx:latest", []string{"8080:80"})

		if err := AddServiceToCompose(fs, path, "web", service); err != nil {
			t.Fatalf("AddServiceToCompose: %v", err)
		}

		result, err := ReadComposeService(fs, path, "web")
		if err != nil {
			t.Fatalf("ReadComposeService: %v", err)
		}
		if ReadServiceImage(result) != "nginx:latest" {
			t.Fatalf("expected image")
		}
	})

	t.Run("complement__existing_service__returns_error", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		path := "/compose.yaml"
		root := makeComposeRoot(map[string]any{
			"web": makeServiceDef("nginx:latest", nil),
		})

		if err := writeYAML(fs, path, root); err != nil {
			t.Fatalf("writeYAML: %v", err)
		}

		if err := AddServiceToCompose(fs, path, "web", makeServiceDef("nginx:latest", nil)); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("complement__invalid_services_type__returns_error", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		path := "/compose.yaml"
		root := map[string]any{
			"services": "invalid",
		}

		if err := writeYAML(fs, path, root); err != nil {
			t.Fatalf("writeYAML: %v", err)
		}

		if err := AddServiceToCompose(fs, path, "web", makeServiceDef("nginx:latest", nil)); err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestWriteCompose(t *testing.T) {
	t.Run("domain__valid_root__writes_yaml_with_newline", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		path := "/compose.yaml"
		root := makeComposeRoot(map[string]any{
			"web": makeServiceDef("nginx:latest", nil),
		})

		if err := WriteCompose(fs, path, root); err != nil {
			t.Fatalf("WriteCompose: %v", err)
		}

		payload, err := afero.ReadFile(fs, path)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		if len(payload) == 0 || payload[len(payload)-1] != '\n' {
			t.Fatalf("expected trailing newline")
		}
	})
}
