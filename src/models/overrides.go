package models

type TaskOverlay struct {
	Build     string `json:"build,omitempty"`
	Check     string `json:"check,omitempty"`
	Contain   string `json:"contain,omitempty"`
	Publish   string `json:"publish,omitempty"`
	Run       string `json:"run,omitempty"`
	Stop      string `json:"stop,omitempty"`
	Install   string `json:"install,omitempty"`
	Uninstall string `json:"uninstall,omitempty"`
	Add       string `json:"add,omitempty"`
	Setup     string `json:"setup,omitempty"`
	Prebuild  string `json:"prebuild,omitempty"`
}

type OverrideGroup struct {
	Group string      `json:"group"`
	Tasks TaskOverlay `json:"tasks"`
}

type GroupSelection struct {
	Group  string
	IsBase bool
}

type ResolvedCommand struct {
	Task     string
	Group    string
	Source   string
	Template string
	Command  string
}

type CacheSlot struct {
	Group       string `json:"group,omitempty"`
	BuildHash   string `json:"build_hash,omitempty"`
	CheckHash   string `json:"check_hash,omitempty"`
	ContainHash string `json:"contain_hash,omitempty"`
	PublishHash string `json:"publish_hash,omitempty"`
}
