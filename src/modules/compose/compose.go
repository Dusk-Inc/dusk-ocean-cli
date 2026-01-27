package compose

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

func ReadComposeRoot(fs afero.Fs, path string) (map[string]any, error) {
	var payload []byte
	if _, err := fs.Stat(path); err != nil {
		if errors.Is(err, afero.ErrFileNotFound) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	content, err := afero.ReadFile(fs, path)
	if err != nil {
		return nil, err
	}
	payload = content
	if len(payload) == 0 {
		return map[string]any{}, nil
	}
	var root map[string]any
	if err := yaml.Unmarshal(payload, &root); err != nil {
		return nil, err
	}
	if root == nil {
		return map[string]any{}, nil
	}
	return root, nil
}

func ReadComposeServices(fs afero.Fs, path string) (map[string]map[string]any, error) {
	root, err := ReadComposeRoot(fs, path)
	if err != nil {
		return nil, err
	}
	servicesRaw, ok := root["services"]
	if !ok {
		return map[string]map[string]any{}, nil
	}
	services, ok := servicesRaw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid services in %s", path)
	}
	out := make(map[string]map[string]any, len(services))
	for name, raw := range services {
		service, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid service definition in %s: %s", path, name)
		}
		out[name] = service
	}
	return out, nil
}

func ReadComposeService(fs afero.Fs, path string, serviceName string) (map[string]any, error) {
	root, err := ReadComposeRoot(fs, path)
	if err != nil {
		return nil, err
	}
	servicesRaw, ok := root["services"]
	if !ok {
		return nil, fmt.Errorf("missing services")
	}
	services, ok := servicesRaw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid services")
	}
	serviceRaw, ok := services[serviceName]
	if !ok {
		return nil, fmt.Errorf("missing service")
	}
	service, ok := serviceRaw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid service")
	}
	return service, nil
}

func ReadServiceImage(service map[string]any) string {
	value, ok := service["image"]
	if !ok {
		return ""
	}
	image, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(image)
}

func ReadServicePorts(service map[string]any) []string {
	value, ok := service["ports"]
	if !ok {
		return nil
	}
	switch typed := value.(type) {
	case []string:
		return append([]string{}, typed...)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				out = append(out, text)
			}
		}
		return out
	case string:
		return []string{typed}
	default:
		return nil
	}
}

func ReadHostPort(port string) string {
	parts := strings.Split(port, ":")
	if len(parts) < 2 {
		return ""
	}
	host := parts[len(parts)-2]
	host = strings.Split(host, "/")[0]
	return strings.TrimSpace(host)
}

func AddServiceToCompose(fs afero.Fs, path string, serviceName string, serviceDef map[string]any) error {
	root, err := ReadComposeRoot(fs, path)
	if err != nil {
		return err
	}
	servicesRaw, ok := root["services"]
	if !ok {
		servicesRaw = map[string]any{}
		root["services"] = servicesRaw
	}
	services, ok := servicesRaw.(map[string]any)
	if !ok {
		return fmt.Errorf("invalid compose services in %s", path)
	}
	if _, exists := services[serviceName]; exists {
		return fmt.Errorf("service already exists in %s: %s", path, serviceName)
	}
	services[serviceName] = serviceDef
	return WriteCompose(fs, path, root)
}

func WriteCompose(fs afero.Fs, path string, root map[string]any) error {
	payload, err := yaml.Marshal(root)
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	return afero.WriteFile(fs, path, payload, 0o644)
}
