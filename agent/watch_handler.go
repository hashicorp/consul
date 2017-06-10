package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
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
func makeWatchHandler(logOutput io.Writer, params interface{}) watch.HandlerFunc {
	script := params.(string)
	logger := log.New(logOutput, "", log.LstdFlags)
	fn := func(idx uint64, data interface{}) {
		// Create the command
		cmd, err := ExecScript(script)
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
			logger.Printf("[ERR] agent: Failed to encode data for watch '%s': %v", script, err)
			return
		}
		cmd.Stdin = &inp

		// Run the handler
		if err := cmd.Run(); err != nil {
			logger.Printf("[ERR] agent: Failed to invoke watch handler '%s': %v", script, err)
		}

		// Get the output, add a message about truncation
		outputStr := string(output.Bytes())
		if output.TotalWritten() > output.Size() {
			outputStr = fmt.Sprintf("Captured %d of %d bytes\n...\n%s",
				output.Size(), output.TotalWritten(), outputStr)
		}

		// Log the output
		logger.Printf("[DEBUG] agent: watch handler '%s' output: %s", script, outputStr)
	}
	return fn
}
