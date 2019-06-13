// +build linux darwin

package envoy

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

func isHotRestartOption(s string) bool {
	restartOpts := []string{
		"--restart-epoch",
		"--hot-restart-version",
		"--drain-time-s",
		"--parent-shutdown-time-s",
	}
	for _, opt := range restartOpts {
		if s == opt {
			return true
		}
		if strings.HasPrefix(s, opt+"=") {
			return true
		}
	}
	return false
}

func hasHotRestartOption(argSets ...[]string) bool {
	for _, args := range argSets {
		for _, opt := range args {
			if isHotRestartOption(opt) {
				return true
			}
		}
	}
	return false
}

func makeBootstrapPipe(bootstrapJSON []byte) (string, error) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		return dir, err
	}

	pipeFile := filepath.Join(dir, "bootstrap.json")

	err = syscall.Mkfifo(pipeFile, 0600)
	if err != nil {
		return dir, err
	}

	// Create an anonymous pipe for STDIN to the child that will write to the
	// named pipe.
	pr, pw, err := os.Pipe()
	if err != nil {
		return dir, err
	}

	// Write the config
	n, err := pw.Write(bootstrapJSON)
	if err != nil {
		return dir, err
	}
	if n < len(bootstrapJSON) {
		return dir, fmt.Errorf("failed writing boostrap to child STDIN: %s", err)
	}

	err = pw.Close()
	if err != nil {
		return dir, err
	}

	// Start a detached child process that we can securely send the bootstrap via
	// STDIN and have it write to the pipe. Cat can do this.
	var attr = os.ProcAttr{
		Dir: ".",
		Env: os.Environ(),
		Files: []*os.File{
			pr,
			nil,
			nil,
		},
	}
	process, err := os.StartProcess("/bin/bash",
		[]string{"bash", "-c", "cat > " + pipeFile + "; rm -rf " + pipeFile},
		&attr,
	)
	if err != nil {
		return dir, err
	}

	err = process.Release()
	if err != nil {
		return dir, err
	}

	return dir, err
}

func execEnvoy(binary string, prefixArgs, suffixArgs []string, bootstrapJSON []byte) error {
	tmpDir, err := makeBootstrapPipe(bootstrapJSON)
	if err != nil {
		os.RemoveAll(tmpDir)
		return err
	}

	// We default to disabling hot restart because it makes it easier to run
	// multiple envoys locally for testing without them trying to share memory and
	// unix sockets and complain about being different IDs. But if user is
	// actually configuring hot-restart explicitly with the --restart-epoch option
	// then don't disable it!
	disableHotRestart := !hasHotRestartOption(prefixArgs, suffixArgs)

	// First argument needs to be the executable name.
	envoyArgs := []string{binary}
	envoyArgs = append(envoyArgs, prefixArgs...)
	envoyArgs = append(envoyArgs, "--config-path", filepath.Join(tmpDir, "bootstrap.json"))
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
