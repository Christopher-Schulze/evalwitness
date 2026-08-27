//go:build windows

package mutation

import (
	"errors"
	"os"
	"os/exec"
)

func configureValidatorProcess(_ *exec.Cmd) {}

func terminateValidatorProcess(command *exec.Cmd) error {
	if command.Process == nil {
		return nil
	}
	err := command.Process.Kill()
	if errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	return err
}
