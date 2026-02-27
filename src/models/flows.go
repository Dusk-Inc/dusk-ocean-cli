package models

type DependencyKind string

type InstallDependency struct {
	Kind       DependencyKind
	App        string
	Name       string
	Path       string
	InstallCmd string
}

type UninstallDependency struct {
	Kind DependencyKind
	App  string
	Name string
	From string
	Path string
}
