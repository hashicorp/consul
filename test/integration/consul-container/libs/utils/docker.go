package utils

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// DockerExec simply shell out to the docker CLI binary on your host.
func DockerExec(args []string, stdout io.Writer) error {
	return cmdExec("docker", "docker", args, stdout, "")
}

func cmdExec(name, binary string, args []string, stdout io.Writer, dir string) error {
	if binary == "" {
		panic("binary named " + name + " was not detected")
	}
	var errWriter bytes.Buffer

	if stdout == nil {
		stdout = os.Stdout
	}

	cmd := exec.Command(binary, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = stdout
	cmd.Stderr = &errWriter
	cmd.Stdin = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("could not invoke %q: %v : %s", name, err, errWriter.String())
	}

	return nil
}
