package interfaces

import "os/exec"

type CommandRunner interface {
	Run(command *exec.Cmd) error
}

// SystemCommandRunner executes commands using the standard os/exec runtime.
type SystemCommandRunner struct{}

func (SystemCommandRunner) Run(command *exec.Cmd) error {
	return command.Run()
}
