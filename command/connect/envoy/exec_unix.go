// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux || darwin
// +build linux darwin

package envoy

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

func makeBootstrapPipe(bootstrapJSON []byte) (string, error) {
	pipeFile := filepath.Join(os.TempDir(),
		fmt.Sprintf("envoy-%x-bootstrap.json", time.Now().UnixNano()+int64(os.Getpid())))

	err := syscall.Mkfifo(pipeFile, 0600)
	if err != nil {
		return pipeFile, err
	}

	binary, args, err := execArgs("connect", "envoy", "pipe-bootstrap", pipeFile)
	if err != nil {
		return pipeFile, err
	}

	// Exec the pipe-bootstrap internal sub-command which will write the bootstrap
	// from STDIN to the named pipe (once Envoy opens it) and then clean up the
	// file for us.
	cmd := exec.Command(binary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return pipeFile, err
	}
	err = cmd.Start()
	if err != nil {
		return pipeFile, err
	}

	// Write the config
	n, err := stdin.Write(bootstrapJSON)
	// Close STDIN whether it was successful or not
	_ = stdin.Close()
	if err != nil {
		return pipeFile, err
	}
	if n < len(bootstrapJSON) {
		return pipeFile, fmt.Errorf("failed writing boostrap to child STDIN: %s", err)
	}

	// We can't wait for the process since we need to exec into Envoy before it
	// will be able to complete so it will be remain as a zombie until Envoy is
	// killed then will be reaped by the init process (pid 0). This is all a bit
	// gross but the cleanest workaround I can think of for Envoy 1.10 not
	// supporting /dev/fd/<fd> config paths any more. So we are done and leaving
	// the child to run it's course without reaping it.
	return pipeFile, nil
}

func execEnvoy(binary string, prefixArgs, suffixArgs []string, bootstrapJSON []byte) error {
	pipeFile, err := makeBootstrapPipe(bootstrapJSON)
	if err != nil {
		os.RemoveAll(pipeFile)
		return err
	}
	// We don't defer a cleanup since we are about to Exec into Envoy which means
	// defer will never fire. The child process cleans up for us in the happy
	// path.

	// We default to disabling hot restart because it makes it easier to run
	// multiple envoys locally for testing without them trying to share memory and
	// unix sockets and complain about being different IDs. But if user is
	// actually configuring hot-restart explicitly with the --restart-epoch option
	// then don't disable it!
	disableHotRestart := !hasHotRestartOption(prefixArgs, suffixArgs)

	// First argument needs to be the executable name.
	envoyArgs := []string{binary}
	envoyArgs = append(envoyArgs, prefixArgs...)
	envoyArgs = append(envoyArgs, "--config-path", pipeFile)
	if disableHotRestart {
		envoyArgs = append(envoyArgs, "--disable-hot-restart")
	}
	envoyArgs = append(envoyArgs, suffixArgs...)

	// Exec
	if err = unix.Exec(binary, envoyArgs, os.Environ()); err != nil {
		return errors.New("Failed to exec envoy: " + err.Error())
	}

	return nil
}
