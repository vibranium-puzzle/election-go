package election

import (
	"bytes"
	"os/exec"
)

func Exec(cmd *ScriptCommand) (int, string, string, error) {
	command := exec.Command(cmd.Name, cmd.Args...)
	var outputBuf, errBuf bytes.Buffer
	command.Stdout = &outputBuf
	command.Stderr = &errBuf

	if err := command.Run(); err != nil {
		return -1, outputBuf.String(), errBuf.String(), err
	}

	rc := command.ProcessState.ExitCode()

	return rc, outputBuf.String(), errBuf.String(), nil
}
