package agent

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	osexec "os/exec"
	"strconv"

	"github.com/armon/circbuf"
	"github.com/hashicorp/consul/agent/exec"
	"github.com/hashicorp/consul/watch"
	"github.com/hashicorp/go-cleanhttp"
	"golang.org/x/net/context"
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
		var cmd *osexec.Cmd
		var err error

		if len(args) > 0 {
			cmd, err = exec.Subprocess(args)
		} else {
			cmd, err = exec.Script(script)
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

func makeHTTPWatchHandler(logOutput io.Writer, config *watch.HttpHandlerConfig) watch.HandlerFunc {
	logger := log.New(logOutput, "", log.LstdFlags)

	fn := func(idx uint64, data interface{}) {
		trans := cleanhttp.DefaultTransport()

		// Skip SSL certificate verification if TLSSkipVerify is true
		if trans.TLSClientConfig == nil {
			trans.TLSClientConfig = &tls.Config{
				InsecureSkipVerify: config.TLSSkipVerify,
			}
		} else {
			trans.TLSClientConfig.InsecureSkipVerify = config.TLSSkipVerify
		}

		ctx := context.Background()
		ctx, cancel := context.WithTimeout(ctx, config.Timeout)
		defer cancel()

		// Create the HTTP client.
		httpClient := &http.Client{
			Transport: trans,
		}

		// Setup the input
		var inp bytes.Buffer
		enc := json.NewEncoder(&inp)
		if err := enc.Encode(data); err != nil {
			logger.Printf("[ERR] agent: Failed to encode data for http watch '%s': %v", config.Path, err)
			return
		}

		req, err := http.NewRequest(config.Method, config.Path, &inp)
		if err != nil {
			logger.Printf("[ERR] agent: Failed to setup http watch: %v", err)
			return
		}
		req = req.WithContext(ctx)
		req.Header.Add("Content-Type", "application/json")
		req.Header.Add("X-Consul-Index", strconv.FormatUint(idx, 10))
		for key, values := range config.Header {
			for _, val := range values {
				req.Header.Add(key, val)
			}
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			logger.Printf("[ERR] agent: Failed to invoke http watch handler '%s': %v", config.Path, err)
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
			logger.Printf("[TRACE] agent: http watch handler '%s' output: %s", config.Path, outputStr)
		} else {
			logger.Printf("[ERR] agent: http watch handler '%s' got '%s' with output: %s",
				config.Path, resp.Status, outputStr)
		}
	}
	return fn
}
