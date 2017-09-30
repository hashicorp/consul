// +build !windows

package agent

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"log"
)

// ExecScript returns a command to execute a script through a shell.
func ExecScript(script string) (*exec.Cmd, error) {
	shell := "/bin/sh"
	if other := os.Getenv("SHELL"); other != "" {
		shell = other
	}
	return exec.Command(shell, "-c", script), nil
}

// ExecSubprocess returns a command to execute a subprocess directly.
func ExecSubprocess(args []string) (*exec.Cmd, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("need an executable to run")
	}

	return exec.Command(args[0], args[1:]...), nil
}

// StartSubprocess starts the given command and sets up signal relaying.
func StartSubprocess(cmd *exec.Cmd, echoSignals bool, logger *log.Logger) error {
	if err := cmd.Start(); err != nil {
		return err
	}

	// If enabled, start a goroutine to relay signals to the subprocess and
	// another to watch for its shutdown.
	if echoSignals {
		signalCh := make(chan os.Signal, 4)
		shutdownCh := make(chan struct{}, 0)
		signal.Notify(signalCh)

		go func() {
			for {
				select {
				case sig := <-signalCh:
					if err := cmd.Process.Signal(sig); err != nil {
						if logger != nil {
							logger.Printf("[ERR] agent: error relaying signal to subprocess: %v", err)
						} else {
							fmt.Println("Error relaying signal to subprocess: ", err)
						}
					}
				case <-shutdownCh:
					signal.Stop(signalCh)
					return
				}
			}
		}()

		go func() {
			cmd.Process.Wait()
			close(shutdownCh)
		}()
	}

	return nil
}
