package functions

import (
	"strings"

	oceanerrors "github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/errors"
	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/models"
)

const baseSlotKey = "__base__"

func lifecycleTaskNames() []string {
	return []string{
		"build", "check", "contain", "publish", "run", "stop",
		"install", "uninstall", "add", "setup", "prebuild",
	}
}

func overlayCommand(overlay models.TaskOverlay, task string) (string, bool) {
	switch task {
	case "build":
		return overlay.Build, overlay.Build != ""
	case "check":
		return overlay.Check, overlay.Check != ""
	case "contain":
		return overlay.Contain, overlay.Contain != ""
	case "publish":
		return overlay.Publish, overlay.Publish != ""
	case "run":
		return overlay.Run, overlay.Run != ""
	case "stop":
		return overlay.Stop, overlay.Stop != ""
	case "install":
		return overlay.Install, overlay.Install != ""
	case "uninstall":
		return overlay.Uninstall, overlay.Uninstall != ""
	case "add":
		return overlay.Add, overlay.Add != ""
	case "setup":
		return overlay.Setup, overlay.Setup != ""
	case "prebuild":
		return overlay.Prebuild, overlay.Prebuild != ""
	default:
		return "", false
	}
}

func overlayTaskNames(overlay models.TaskOverlay) []string {
	var names []string
	for _, task := range lifecycleTaskNames() {
		if _, ok := overlayCommand(overlay, task); ok {
			names = append(names, task)
		}
	}
	return names
}

func baseTaskDeclared(config models.RepoConfig, task string) bool {
	cmd, err := RepoCommand(config, task)
	if err != nil {
		return false
	}
	return strings.TrimSpace(cmd) != ""
}

func ValidateOverrides(config models.RepoConfig) ([]models.OverrideGroup, error) {
	seen := map[string]struct{}{}
	for _, group := range config.Overrides {
		if group.Group == "" {
			return nil, oceanerrors.NewEmptyGroupNameError()
		}
		if _, dup := seen[group.Group]; dup {
			return nil, oceanerrors.NewDuplicateGroupError(group.Group)
		}
		seen[group.Group] = struct{}{}
		for _, task := range overlayTaskNames(group.Tasks) {
			if !baseTaskDeclared(config, task) {
				return nil, oceanerrors.NewUnknownBaseTaskError(group.Group, task)
			}
		}
	}
	return config.Overrides, nil
}

func SelectGroup(group string) models.GroupSelection {
	if group == "" {
		return models.GroupSelection{IsBase: true}
	}
	return models.GroupSelection{Group: group}
}

func findGroup(groups []models.OverrideGroup, name string) (models.OverrideGroup, bool) {
	for _, g := range groups {
		if g.Group == name {
			return g, true
		}
	}
	return models.OverrideGroup{}, false
}

func groupNames(groups []models.OverrideGroup) []string {
	names := make([]string, 0, len(groups))
	for _, g := range groups {
		names = append(names, g.Group)
	}
	return names
}

func ResolveGroupCommand(task string, selection models.GroupSelection, config models.RepoConfig, groups []models.OverrideGroup, ctx VariableContext) (models.ResolvedCommand, error) {
	baseTemplate, err := RepoCommand(config, task)
	if err != nil {
		return models.ResolvedCommand{}, err
	}

	resolved := models.ResolvedCommand{Task: task, Source: "base", Template: baseTemplate}

	if !selection.IsBase {
		group, ok := findGroup(groups, selection.Group)
		if !ok {
			return models.ResolvedCommand{}, oceanerrors.NewUnknownGroupError(selection.Group, groupNames(groups))
		}
		resolved.Group = selection.Group
		if overlay, has := overlayCommand(group.Tasks, task); has {
			resolved.Source = "group"
			resolved.Template = overlay
		}
	}

	command, err := Substitute(resolved.Template, ctx)
	if err != nil {
		return models.ResolvedCommand{}, err
	}
	resolved.Command = command
	return resolved, nil
}

func slotKey(selection models.GroupSelection) string {
	if selection.IsBase {
		return baseSlotKey
	}
	return selection.Group
}

func cacheableTask(task string) bool {
	switch task {
	case "build", "check", "contain", "publish":
		return true
	default:
		return false
	}
}

func slotHash(slot models.CacheSlot, task string) (string, bool) {
	switch task {
	case "build":
		return slot.BuildHash, slot.BuildHash != ""
	case "check":
		return slot.CheckHash, slot.CheckHash != ""
	case "contain":
		return slot.ContainHash, slot.ContainHash != ""
	case "publish":
		return slot.PublishHash, slot.PublishHash != ""
	default:
		return "", false
	}
}

func withSlotHash(slot models.CacheSlot, task, hash string) models.CacheSlot {
	switch task {
	case "build":
		slot.BuildHash = hash
	case "check":
		slot.CheckHash = hash
	case "contain":
		slot.ContainHash = hash
	case "publish":
		slot.PublishHash = hash
	}
	return slot
}
