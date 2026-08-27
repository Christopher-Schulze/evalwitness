//go:build windows

package main

import (
	"os"
	"os/exec"
)

func configureBonProcessGroup(command *exec.Cmd) {
	command.Cancel = func() error {
		if command.Process == nil {
			return os.ErrProcessDone
		}
		return command.Process.Kill()
	}
}
