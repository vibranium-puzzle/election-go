package shell

import (
	"bytes"
	"errors"
	"os/exec"
)

func Exec(cmd string) (int, string, string, error) {
	command := exec.Command("sh", "-c", cmd)
	inputContent, inputErr := command.Output()
	errorContent, errorErr := command.CombinedOutput()

	stdout := string(bytes.TrimSpace(inputContent))
	stderr := string(bytes.TrimSpace(errorContent))

	if inputErr != nil || errorErr != nil {
		err := errors.New("failed to execute command")
		return -1, stdout, stderr, err
	}

	rc := command.ProcessState.ExitCode()

	return rc, stdout, stderr, nil
}
