package runner

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/hashicorp/go-hclog"
)

type Runner struct {
	logger hclog.Logger

	tfBin     string
	dockerBin string
}

func Load(logger hclog.Logger) (*Runner, error) {
	r := &Runner{
		logger: logger,
	}

	type item struct {
		name string
		dest *string
		warn string // optional
	}
	lookup := []item{
		{"docker", &r.dockerBin, ""},
		{"terraform", &r.tfBin, ""},
	}

	var (
		bins []string
		err  error
	)
	for _, i := range lookup {
		*i.dest, err = exec.LookPath(i.name)
		if err != nil {
			if errors.Is(err, exec.ErrNotFound) {
				if i.warn != "" {
					return nil, fmt.Errorf("Could not find %q on path (%s): %w", i.name, i.warn, err)
				} else {
					return nil, fmt.Errorf("Could not find %q on path: %w", i.name, err)
				}
			}
			return nil, fmt.Errorf("Unexpected failure looking for %q on path: %w", i.name, err)
		}
		bins = append(bins, *i.dest)
	}
	r.logger.Trace("using binaries", "paths", bins)

	return r, nil
}

func (r *Runner) DockerExec(args []string, stdout io.Writer, stdin io.Reader) error {
	return cmdExec("docker", r.dockerBin, args, stdout, stdin, "")
}

func (r *Runner) TerraformExec(args []string, stdout io.Writer, workdir string) error {
	return cmdExec("terraform", r.tfBin, args, stdout, nil, workdir)
}

func cmdExec(name, binary string, args []string, stdout io.Writer, stdin io.Reader, dir string) error {
	if binary == "" {
		panic("binary named " + name + " was not detected")
	}
	var errWriter bytes.Buffer

	if stdout == nil {
		stdout = os.Stdout // TODO: wrap logs
	}

	cmd := exec.Command(binary, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = stdout
	cmd.Stderr = &errWriter
	cmd.Stdin = stdin
	if err := cmd.Run(); err != nil {
		return &ExecError{
			BinaryName:  name,
			Err:         err,
			ErrorOutput: errWriter.String(),
		}
	}

	return nil
}

type ExecError struct {
	BinaryName  string
	ErrorOutput string
	Err         error
}

func (e *ExecError) Unwrap() error {
	return e.Err
}

func (e *ExecError) Error() string {
	return fmt.Sprintf(
		"could not invoke %q: %v : %s",
		e.BinaryName,
		e.Err,
		e.ErrorOutput,
	)
}
