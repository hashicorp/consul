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
	"github.com/hashicorp/go-cleanhttp"
	"net/http"
	"time"
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

func makeHTTPWatchHandler(logOutput io.Writer, method string, httpUrl string, headers map[string][]string, timeout time.Duration) watch.HandlerFunc {
	logger := log.New(logOutput, "", log.LstdFlags)

	fn := func(idx uint64, data interface{}) {
		// Create the transport. We disable HTTP Keep-Alive's to prevent
		// failing checks due to the keepalive interval.
		trans := cleanhttp.DefaultTransport()
		trans.DisableKeepAlives = true

		// TODO: put in outer scope
		// Create the HTTP client.
		httpClient := &http.Client{
			Timeout:   timeout,
			Transport: trans,
		}

		// Setup the input
		var inp bytes.Buffer
		enc := json.NewEncoder(&inp)
		if err := enc.Encode(data); err != nil {
			logger.Printf("[ERR] agent: Failed to encode data for http watch '%s': %v", httpUrl, err)
			return
		}

		req, err := http.NewRequest(method, httpUrl, &inp)
		if err != nil {
			logger.Printf("[ERR] agent: Failed to setup http watch: %v", err)
			return
		}
		for key, values := range headers {
			for _, val := range values {
				req.Header.Add(key, val)
			}
		}
		req.Header.Add("X-Consul-Index", strconv.FormatUint(idx, 10))
		resp, err := httpClient.Do(req)
		if err != nil {
			logger.Printf("[ERR] agent: Failed to invoke http watch handler '%s': %v", httpUrl, err)
			return
		}
		defer resp.Body.Close()

		// Collect the output
		output, _ := circbuf.NewBuffer(WatchBufSize)
		io.Copy(output, resp.Body)

		// Get the output, add a message about truncation
		outputStr := string(output.Bytes())
		if output.TotalWritten() > output.Size() {
			outputStr = fmt.Sprintf("Captured %d of %d bytes\n...\n%s",
				output.Size(), output.TotalWritten(), outputStr)
		}

		if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
			// Log the output
			logger.Printf("[DEBUG] agent: http watch handler '%s' output: %s", httpUrl, outputStr)
		} else {
			logger.Printf("[ERR] agent: http watch handler '%s' got '%s' with output: %s",
				httpUrl, resp.Status, outputStr)
		}
	}
	return fn
}
