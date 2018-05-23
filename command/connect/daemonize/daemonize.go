package daemonize

import (
	"fmt"
	_ "net/http/pprof"
	"os" // Expose pprof if configured
	"os/exec"
	"syscall"

	"github.com/mitchellh/cli"
)

func New(ui cli.Ui) *cmd {
	return &cmd{UI: ui}
}

type cmd struct {
	UI cli.Ui

	stdoutPath string
	stderrPath string
	cmdArgs    []string
}

func (c *cmd) Run(args []string) int {
	numArgs := len(args)
	if numArgs < 3 {
		c.UI.Error("Need at least 3 arguments; stdoutPath, stdinPath, " +
			"executablePath [arguments...]")
		os.Exit(1)
	}

	c.stdoutPath, c.stderrPath = args[0], args[1]
	c.cmdArgs = args[2:] // includes the executable as arg 0 as expected

	// Open log files if specified
	var stdoutF, stderrF *os.File
	var err error
	if c.stdoutPath != "_" {
		stdoutF, err = os.OpenFile(c.stdoutPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			c.UI.Error(fmt.Sprintf("error creating stdout file: %s", err))
			return 1
		}
	}
	if c.stderrPath != "_" {
		stderrF, err = os.OpenFile(c.stderrPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			c.UI.Error(fmt.Sprintf("error creating stderr file: %s", err))
			return 1
		}
	}

	// Exec the command passed in a new session then exit to ensure it's adopted
	// by the init process. Use the passed file paths for std out/err.
	cmd := &exec.Cmd{
		Path: c.cmdArgs[0],
		Args: c.cmdArgs,
		// Inherit Dir and Env by default.
		SysProcAttr: &syscall.SysProcAttr{Setsid: true},
	}
	cmd.Stdin = nil
	cmd.Stdout = stdoutF
	cmd.Stderr = stderrF

	// Exec the child
	err = cmd.Start()
	if err != nil {
		c.UI.Error("command failed with error: " + err.Error())
		os.Exit(1)
	}

	// Print it's PID to stdout
	fmt.Fprintf(os.Stdout, "%d\n", cmd.Process.Pid)

	// Release (no-op on unix) and exit to orphan the child and get it adopted by
	// init.
	cmd.Process.Release()
	return 0
}

func (c *cmd) Synopsis() string {
	return ""
}

func (c *cmd) Help() string {
	return ""
}
