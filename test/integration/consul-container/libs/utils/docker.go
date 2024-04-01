// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package utils

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/hashicorp/go-version"
)

// DockerExec simply shell out to the docker CLI binary on your host.
func DockerExec(args []string, stdout io.Writer) error {
	return cmdExec("docker", "docker", args, stdout, "")
}

// DockerImageVersion retrieves the value of the org.opencontainers.image.version label from the specified image.
func DockerImageVersion(imageName string) (*version.Version, error) {
	var b strings.Builder
	err := cmdExec("docker", "docker", []string{"image", "inspect", "--format", `{{index .Config.Labels "org.opencontainers.image.version"}}`, imageName}, &b, "")
	if err != nil {
		return nil, err
	}
	output := b.String()

	return version.NewVersion(strings.TrimSpace(output))
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
