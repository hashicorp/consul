// +build !windows

package core

import (
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"
)

// trapSignalsPosix captures POSIX-only signals.
func trapSignalsPosix() {
	go func() {
		sigchan := make(chan os.Signal, 1)
		signal.Notify(sigchan, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGUSR1)

		for sig := range sigchan {
			switch sig {
			case syscall.SIGTERM:
				log.Println("[INFO] SIGTERM: Terminating process")
				if PidFile != "" {
					os.Remove(PidFile)
				}
				os.Exit(0)

			case syscall.SIGQUIT:
				log.Println("[INFO] SIGQUIT: Shutting down")
				exitCode := executeShutdownCallbacks("SIGQUIT")
				err := Stop()
				if err != nil {
					log.Printf("[ERROR] SIGQUIT stop: %v", err)
					exitCode = 1
				}
				if PidFile != "" {
					os.Remove(PidFile)
				}
				os.Exit(exitCode)

			case syscall.SIGHUP:
				log.Println("[INFO] SIGHUP: Hanging up")
				err := Stop()
				if err != nil {
					log.Printf("[ERROR] SIGHUP stop: %v", err)
				}

			case syscall.SIGUSR1:
				log.Println("[INFO] SIGUSR1: Reloading")

				var updatedCorefile Input

				corefileMu.Lock()
				if corefile == nil {
					// Hmm, did spawing process forget to close stdin? Anyhow, this is unusual.
					log.Println("[ERROR] SIGUSR1: no Corefile to reload (was stdin left open?)")
					corefileMu.Unlock()
					continue
				}
				if corefile.IsFile() {
					body, err := ioutil.ReadFile(corefile.Path())
					if err == nil {
						updatedCorefile = CorefileInput{
							Filepath: corefile.Path(),
							Contents: body,
							RealFile: true,
						}
					}
				}
				corefileMu.Unlock()

				err := Restart(updatedCorefile)
				if err != nil {
					log.Printf("[ERROR] SIGUSR1: %v", err)
				}
			}
		}
	}()
}
