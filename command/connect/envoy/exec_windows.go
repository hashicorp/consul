//go:build windows
//go:build !fips
// +build windows
// +build !fips

package envoy

import (
	"errors"
	"fmt"
	"github.com/natefinch/npipe"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func makeBootstrapPipe(bootstrapJSON []byte) (string, error) {
	pipeFile := filepath.Join(os.TempDir(),
		fmt.Sprintf("envoy-%x-bootstrap.json", time.Now().UnixNano()+int64(os.Getpid())))

	binary, args, err := execArgs("connect", "envoy", "pipe-bootstrap", pipeFile)
	if err != nil {
		return pipeFile, err
	}

	// Dial the named pipe
	pipeConn, err := npipe.Dial(pipeFile)
	if err != nil {
		return pipeFile, err
	}
	defer pipeConn.Close()

	// Start the command to connect to the named pipe
	cmd := exec.Command(binary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = pipeConn

	// Start the command
	err = cmd.Start()
	if err != nil {
		return pipeFile, err
	}

	// Write the config
	n, err := pipeConn.Write(bootstrapJSON)
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

func startProc(binary string, args []string) (p *os.Process, err error) {
	if binary, err = exec.LookPath(binary); err == nil {
		var procAttr os.ProcAttr
		procAttr.Files = []*os.File{os.Stdin,
			os.Stdout, os.Stderr}
		p, err := os.StartProcess(binary, args, &procAttr)
		if err == nil {
			return p, nil
		}
	}
	return nil, err
}

func execEnvoy(binary string, prefixArgs, suffixArgs []string, bootstrapJSON []byte) error {
	tempFile, err := makeBootstrapPipe(bootstrapJSON)
	if err != nil {
		os.RemoveAll(tempFile)
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
	envoyArgs := []string{}
	envoyArgs = append(envoyArgs, prefixArgs...)
	if disableHotRestart {
		envoyArgs = append(envoyArgs, "--disable-hot-restart")
	}
	envoyArgs = append(envoyArgs, suffixArgs...)
	envoyArgs = append(envoyArgs, "--config-path", tempFile)

	// Exec
	if proc, err := startProc(binary, envoyArgs); err == nil {
		proc.Wait()
	} else if err != nil {
		return errors.New("Failed to exec envoy: " + err.Error())
	}

	return nil
}
