package interfaces

import "os/exec"

type CommandRunner interface {
	Run(command *exec.Cmd) error
}

type SystemCommandRunner struct{}

func (SystemCommandRunner) Run(command *exec.Cmd) error {
	return command.Run()
}
