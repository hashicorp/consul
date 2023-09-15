// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package exec

import (
	"fmt"
	"os/exec"
)

// Subprocess returns a command to execute a subprocess directly.
func Subprocess(args []string) (*exec.Cmd, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("need an executable to run")
	}
	return exec.Command(args[0], args[1:]...), nil
}
