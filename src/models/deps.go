package models

type DependencyNodeKind string

type DependencyNode struct {
	Kind DependencyNodeKind
	App  string
	Name string
	Deps []WorkspaceDep
}
