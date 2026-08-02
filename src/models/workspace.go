package models

import "github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"

type WorkspaceConfig struct {
	Workspace      string              `json:"workspace"`
	Version        string              `json:"version,omitempty"`
	Variables      map[string]string   `json:"variables"`
	Tasks          map[string]string   `json:"tasks"`
	Ports          WorkspacePorts      `json:"ports"`
	Apps           []WorkspaceApp      `json:"apps"`
	Libraries      []WorkspaceLibrary  `json:"libraries"`
	Projects       []WorkspaceProject  `json:"projects"`
	Templates      []WorkspaceTemplate `json:"templates"`
	Infrastructure []WorkspaceInfra    `json:"infrastructure"`
	Docs           []WorkspaceDocs     `json:"docs"`
}

type WorkspacePorts struct {
	Allowed  WorkspacePortRange      `json:"allowed"`
	Reserved []WorkspaceReservedPort `json:"reserved"`
}

type WorkspacePortRange struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

type WorkspaceReservedPort struct {
	Name string `json:"name"`
	Port int    `json:"port"`
}

type WorkspaceApp struct {
	Name      string             `json:"name"`
	Remote    string             `json:"remote,omitempty"`
	Variables map[string]string  `json:"variables,omitempty"`
	Services  []WorkspaceService `json:"services"`
	Libraries []WorkspaceLibrary `json:"libraries"`
	Projects  []WorkspaceProject `json:"projects"`
	Testing   []WorkspaceTest    `json:"testing"`
}

type WorkspaceImage struct {
	Name string `json:"name"`
	Tag  string `json:"tag"`
}

type WorkspaceService struct {
	Name          string            `json:"name"`
	Port          string            `json:"port"`
	Image         WorkspaceImage    `json:"image"`
	Dockerfile    string            `json:"Dockerfile"`
	ContainerFile string            `json:"container_file,omitempty"`
	ImagePath     string            `json:"image_path,omitempty"`
	Remote        string            `json:"remote,omitempty"`
	Variables     map[string]string `json:"variables,omitempty"`
	Scopes        []string          `json:"scopes,omitempty"`
	Deps          []WorkspaceDep    `json:"deps"`
}

type WorkspaceLibrary struct {
	Name      string            `json:"name"`
	Remote    string            `json:"remote,omitempty"`
	Variables map[string]string `json:"variables,omitempty"`
	Scopes    []string          `json:"scopes,omitempty"`
	Deps      []WorkspaceDep    `json:"deps"`
}

type WorkspaceProject struct {
	Name      string            `json:"name"`
	Remote    string            `json:"remote,omitempty"`
	Variables map[string]string `json:"variables,omitempty"`
	Scopes    []string          `json:"scopes,omitempty"`
	Deps      []WorkspaceDep    `json:"deps"`
}

type WorkspaceTest struct {
	Name      string            `json:"name"`
	Remote    string            `json:"remote,omitempty"`
	Variables map[string]string `json:"variables,omitempty"`
	Scopes    []string          `json:"scopes,omitempty"`
	Deps      []WorkspaceDep    `json:"deps"`
}

type WorkspaceTemplate struct {
	Name      string            `json:"name"`
	Kind      string            `json:"kind"`
	Remote    string            `json:"remote,omitempty"`
	Variables map[string]string `json:"variables,omitempty"`
	Deps      []WorkspaceDep    `json:"deps"`
}

type WorkspaceInfra struct {
	Name      string            `json:"name"`
	Remote    string            `json:"remote,omitempty"`
	Variables map[string]string `json:"variables,omitempty"`
}

type WorkspaceDocs struct {
	Name      string            `json:"name"`
	Remote    string            `json:"remote,omitempty"`
	Variables map[string]string `json:"variables,omitempty"`
}

type WorkspaceDep struct {
	Lib  string `json:"lib"`
	From string `json:"from"`
}

type RepoConfig struct {
	Name      string   `json:"name"`
	Language  string   `json:"language"`
	Type      string   `json:"type"`
	Build     string   `json:"build"`
	Test      string   `json:"test"`
	Add       string   `json:"add"`
	Install   string   `json:"install"`
	Uninstall string   `json:"uninstall"`
	Scopes    []string `json:"scopes,omitempty"`
	Tasks     struct {
		Build     string `json:"build"`
		Check     string `json:"check"`
		Test      string `json:"test"`
		Add       string `json:"add"`
		Install   string `json:"install"`
		Uninstall string `json:"uninstall"`
		Contain   string `json:"contain"`
		Publish   string `json:"publish"`
		Run       string `json:"run"`
		Stop      string `json:"stop"`
		Setup     string `json:"setup"`
		Prebuild  string `json:"prebuild"`
	} `json:"tasks"`
	Overrides []OverrideGroup `json:"overrides,omitempty"`
}

type TargetKind string

const (
	TargetApp        TargetKind = tokens.TargetAppKind
	TargetService    TargetKind = tokens.TargetServiceKind
	TargetAppLib     TargetKind = tokens.TargetAppLibKind
	TargetGlobalLib  TargetKind = tokens.TargetGlobalLibKind
	TargetProject    TargetKind = tokens.TargetProjectKind
	TargetAppProject TargetKind = tokens.TargetAppProjectKind
	TargetTest       TargetKind = tokens.TargetTestKind
	TargetTemplate   TargetKind = tokens.TargetTemplateKind
)

type Target struct {
	Kind TargetKind
	App  string
	Name string
	Path string
}

type InitOptions struct {
	Name string
}
