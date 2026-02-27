package functions

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/afero"
)

func NextServicePort(fs afero.Fs, appName string) (string, error) {
	workspaceConfig, err := ReadWorkspaceConfig(fs)
	if err != nil {
		return "", err
	}
	startPort := workspaceConfig.Ports.Allowed.Min
	endPort := workspaceConfig.Ports.Allowed.Max
	reserved := map[int]struct{}{}
	for _, entry := range workspaceConfig.Ports.Reserved {
		reserved[entry.Port] = struct{}{}
	}
	maxPort := startPort - 1
	for _, app := range workspaceConfig.Apps {
		if app.Name != appName {
			continue
		}
		for _, service := range app.Services {
			value, err := strconv.Atoi(service.Port)
			if err != nil {
				continue
			}
			if value < startPort || value > endPort {
				continue
			}
			if value > maxPort {
				maxPort = value
			}
		}
	}
	if maxPort < startPort {
		maxPort = startPort - 1
	}
	candidate := maxPort + 1
	for candidate <= endPort {
		if _, blocked := reserved[candidate]; blocked {
			candidate++
			continue
		}
		return strconv.Itoa(candidate), nil
	}
	return "", fmt.Errorf("no available ports in allowed range %d-%d", startPort, endPort)
}

func DefaultServiceImage(appName string, serviceName string) WorkspaceImage {
	return WorkspaceImage{
		Name: fmt.Sprintf("%s__%s", appName, serviceName),
		Tag:  "dev",
	}
}

func ServiceImageReference(fs afero.Fs, appName string, serviceName string) (string, error) {
	workspaceConfig, err := ReadWorkspaceConfig(fs)
	if err != nil {
		return "", err
	}
	image := DefaultServiceImage(appName, serviceName)
	for _, app := range workspaceConfig.Apps {
		if app.Name != appName {
			continue
		}
		for _, service := range app.Services {
			if service.Name != serviceName {
				continue
			}
			if service.Image.Name != "" {
				image.Name = service.Image.Name
			}
			if service.Image.Tag != "" {
				image.Tag = service.Image.Tag
			}
		}
	}
	return formatImageReference(image), nil
}

func AddServiceToWorkspace(fs afero.Fs, appName string, serviceName string, port string, image WorkspaceImage, dockerfile string) error {
	workspaceConfig, err := ReadWorkspaceConfig(fs)
	if err != nil {
		return err
	}

	imageDefaults := DefaultServiceImage(appName, serviceName)
	if image.Name == "" {
		image.Name = imageDefaults.Name
	}
	if image.Tag == "" {
		image.Tag = imageDefaults.Tag
	}

	appIndex := FindAppIndex(workspaceConfig, appName)
	if appIndex == -1 {
		workspaceConfig.Apps = append(workspaceConfig.Apps, WorkspaceApp{
			Name: appName,
			Services: []WorkspaceService{
				{
					Name:       serviceName,
					Port:       port,
					Image:      image,
					Dockerfile: dockerfile,
					Deps:       []WorkspaceDep{},
				},
			},
			Libraries: []WorkspaceLibrary{},
			Testing:   []WorkspaceTest{},
		})
		return WriteWorkspaceConfig(fs, workspaceConfig)
	}

	serviceIndex := FindServiceIndex(workspaceConfig.Apps[appIndex], serviceName)
	if serviceIndex == -1 {
		workspaceConfig.Apps[appIndex].Services = append(workspaceConfig.Apps[appIndex].Services, WorkspaceService{
			Name:       serviceName,
			Port:       port,
			Image:      image,
			Dockerfile: dockerfile,
			Deps:       []WorkspaceDep{},
		})
	} else {
		existingDeps := workspaceConfig.Apps[appIndex].Services[serviceIndex].Deps
		existingDockerfile := workspaceConfig.Apps[appIndex].Services[serviceIndex].Dockerfile
		if strings.TrimSpace(dockerfile) == "" {
			dockerfile = existingDockerfile
		}
		workspaceConfig.Apps[appIndex].Services[serviceIndex] = WorkspaceService{
			Name:       serviceName,
			Port:       port,
			Image:      image,
			Dockerfile: dockerfile,
			Deps:       existingDeps,
		}
	}

	return WriteWorkspaceConfig(fs, workspaceConfig)
}

// RemoveServiceFromWorkspace removes a service entry from ocean.workspace.json.
func RemoveServiceFromWorkspace(fs afero.Fs, appName string, serviceName string) error {
	return UpdateConfig(fs, func(config WorkspaceConfig) (WorkspaceConfig, error) {
		appIndex := FindAppIndex(config, appName)
		if appIndex == -1 {
			return config, nil
		}
		serviceIndex := FindServiceIndex(config.Apps[appIndex], serviceName)
		if serviceIndex == -1 {
			return config, nil
		}
		services := config.Apps[appIndex].Services
		config.Apps[appIndex].Services = append(services[:serviceIndex], services[serviceIndex+1:]...)
		return config, nil
	})
}

func MakeServiceNode(config WorkspaceConfig, appName string, name string) (Node, error) {
	appIndex := FindAppIndex(config, appName)
	if appIndex == -1 {
		return Node{}, fmt.Errorf("app not registered in workspace: %s", appName)
	}
	serviceIndex := FindServiceIndex(config.Apps[appIndex], name)
	if serviceIndex == -1 {
		return Node{}, fmt.Errorf("service not registered in workspace: %s", name)
	}
	service := config.Apps[appIndex].Services[serviceIndex]
	return Node{
		Kind: NodeService,
		App:  appName,
		Name: name,
		Deps: service.Deps,
	}, nil
}

func formatImageReference(image WorkspaceImage) string {
	name := strings.TrimSpace(image.Name)
	if name == "" {
		return ""
	}
	tag := strings.TrimSpace(image.Tag)
	if tag != "" {
		name = fmt.Sprintf("%s:%s", name, tag)
	}
	return name
}
