// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package cluster

import (
	"context"
	"fmt"
	"io"

	tcexec "github.com/testcontainers/testcontainers-go/exec"
)

// ExecResult is the outcome of a command run inside an agent container.
type ExecResult struct {
	ExitCode int
	// Output is the demultiplexed output. It only contains stdout unless the
	// command was run with ExecCombined.
	Output string
}

// ExecStdout runs cmd inside the agent's Consul container and returns only
// its stdout, so that machine-readable output (e.g. JSON) is not polluted by
// warnings written to stderr. A non-zero exit code is reported in the result,
// not as an error.
func ExecStdout(ctx context.Context, agent Agent, cmd ...string) (ExecResult, error) {
	return execDemuxed(ctx, agent, append([]string{"sh", "-c", `exec "$@" 2>/dev/null`, "sh"}, cmd...))
}

// ExecCombined runs cmd inside the agent's Consul container and returns
// stdout and stderr interleaved. Use it to assert on error messages.
func ExecCombined(ctx context.Context, agent Agent, cmd ...string) (ExecResult, error) {
	return execDemuxed(ctx, agent, cmd)
}

func execDemuxed(ctx context.Context, agent Agent, cmd []string) (ExecResult, error) {
	code, reader, err := agent.GetConsulContainer().Exec(ctx, cmd, tcexec.Multiplexed())
	if err != nil {
		return ExecResult{}, fmt.Errorf("exec %v: %w", cmd, err)
	}
	out, err := io.ReadAll(reader)
	if err != nil {
		return ExecResult{}, fmt.Errorf("reading exec output: %w", err)
	}
	return ExecResult{ExitCode: code, Output: string(out)}, nil
}
