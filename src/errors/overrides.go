package errors

import (
	"fmt"
	"strings"
)

const (
	KindDuplicateGroup = "duplicate_group"
	KindEmptyGroupName = "empty_group_name"
	KindUnknownBaseTask = "unknown_base_task"
)

type OverridesValidationError struct {
	Kind    string
	Group   string
	Task    string
	Message string
}

func (e *OverridesValidationError) Error() string {
	return e.Message
}

func NewDuplicateGroupError(group string) *OverridesValidationError {
	return &OverridesValidationError{
		Kind:    KindDuplicateGroup,
		Group:   group,
		Message: fmt.Sprintf("overrides: duplicate group name %q", group),
	}
}

func NewEmptyGroupNameError() *OverridesValidationError {
	return &OverridesValidationError{
		Kind:    KindEmptyGroupName,
		Message: "overrides: group entry has a missing or empty name",
	}
}

func NewUnknownBaseTaskError(group, task string) *OverridesValidationError {
	return &OverridesValidationError{
		Kind:    KindUnknownBaseTask,
		Group:   group,
		Task:    task,
		Message: fmt.Sprintf("overrides: group %q overlays unknown base task %q", group, task),
	}
}

type UnknownGroupError struct {
	Group   string
	Known   []string
	Message string
}

func (e *UnknownGroupError) Error() string {
	return e.Message
}

func NewUnknownGroupError(group string, known []string) *UnknownGroupError {
	msg := fmt.Sprintf("unknown --group %q", group)
	if len(known) > 0 {
		msg = fmt.Sprintf("%s; known groups: %s", msg, strings.Join(known, ", "))
	} else {
		msg = fmt.Sprintf("%s; no override groups declared", msg)
	}
	return &UnknownGroupError{Group: group, Known: known, Message: msg}
}
