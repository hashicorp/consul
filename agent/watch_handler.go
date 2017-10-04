package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"

	"github.com/armon/circbuf"
	"github.com/hashicorp/consul/watch"
)

const (
	// Limit the size of a watch handlers's output to the
	// last WatchBufSize. Prevents an enormous buffer
	// from being captured
	WatchBufSize = 4 * 1024 // 4KB
)

// makeWatchHandler returns a handler for the given watch
func makeWatchHandler(logOutput io.Writer, handler interface{}) watch.HandlerFunc {
	var args []string
	var script string

	// Figure out whether to run in shell or raw subprocess mode
	switch h := handler.(type) {
	case string:
		script = h
	case []string:
		args = h
	default:
		panic(fmt.Errorf("unknown handler type %T", handler))
	}

	logger := log.New(logOutput, "", log.LstdFlags)
	fn := func(idx uint64, data interface{}) {
		// Create the command
		var cmd *exec.Cmd
		var err error

		if len(args) > 0 {
			cmd, err = ExecSubprocess(args)
		} else {
			cmd, err = ExecScript(script)
		}
		if err != nil {
			logger.Printf("[ERR] agent: Failed to setup watch: %v", err)
			return
		}

		cmd.Env = append(os.Environ(),
			"CONSUL_INDEX="+strconv.FormatUint(idx, 10),
		)

		// Collect the output
		output, _ := circbuf.NewBuffer(WatchBufSize)
		cmd.Stdout = output
		cmd.Stderr = output

		// Setup the input
		var inp bytes.Buffer
		enc := json.NewEncoder(&inp)
		if err := enc.Encode(data); err != nil {
			logger.Printf("[ERR] agent: Failed to encode data for watch '%v': %v", handler, err)
			return
		}
		cmd.Stdin = &inp

		// Run the handler
		if err := cmd.Run(); err != nil {
			logger.Printf("[ERR] agent: Failed to run watch handler '%v': %v", handler, err)
		}

		// Get the output, add a message about truncation
		outputStr := string(output.Bytes())
		if output.TotalWritten() > output.Size() {
			outputStr = fmt.Sprintf("Captured %d of %d bytes\n...\n%s",
				output.Size(), output.TotalWritten(), outputStr)
		}

		// Log the output
		logger.Printf("[DEBUG] agent: watch handler '%v' output: %s", handler, outputStr)
	}
	return fn
}
