// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	osexec "os/exec"
	"strconv"

	"github.com/armon/circbuf"
	"github.com/hashicorp/consul/agent/exec"
	"github.com/hashicorp/consul/api/watch"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-hclog"
	"golang.org/x/net/context"
)

const (
	// Limit the size of a watch handlers's output to the
	// last WatchBufSize. Prevents an enormous buffer
	// from being captured
	WatchBufSize = 4 * 1024 // 4KB
)

// makeWatchHandler returns a handler for the given watch
func makeWatchHandler(logger hclog.Logger, handler interface{}) watch.HandlerFunc {
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
			logger.Error("Failed to setup watch", "error", err)
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
			logger.Error("Failed to encode data for watch",
				"watch", handler,
				"error", err,
			)
			return
		}
		cmd.Stdin = &inp

		// Run the handler
		if err := cmd.Run(); err != nil {
			logger.Error("Failed to run watch handler",
				"watch_handler", handler,
				"error", err,
			)
		}

		// Get the output, add a message about truncation
		outputStr := string(output.Bytes())
		if output.TotalWritten() > output.Size() {
			outputStr = fmt.Sprintf("Captured %d of %d bytes\n...\n%s",
				output.Size(), output.TotalWritten(), outputStr)
		}

		// Log the output
		logger.Debug("watch handler output",
			"watch_handler", handler,
			"output", outputStr,
		)
	}
	return fn
}

func makeHTTPWatchHandler(logger hclog.Logger, config *watch.HttpHandlerConfig) watch.HandlerFunc {
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
			logger.Error("Failed to encode data for http watch",
				"watch", config.Path,
				"error", err,
			)
			return
		}

		req, err := http.NewRequest(config.Method, config.Path, &inp)
		if err != nil {
			logger.Error("Failed to setup http watch", "error", err)
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
			logger.Error("Failed to invoke http watch handler",
				"watch", config.Path,
				"error", err,
			)
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
			logger.Trace("http watch handler output",
				"watch", config.Path,
				"output", outputStr,
			)
		} else {
			logger.Error("http watch handler failed with output",
				"watch", config.Path,
				"status", resp.Status,
				"output", outputStr,
			)
		}
	}
	return fn
}

// TODO: return a fully constructed watch.Plan with a Plan.Handler, so that Exempt
// can be ignored by the caller.
func makeWatchPlan(logger hclog.Logger, params map[string]interface{}) (*watch.Plan, error) {
	wp, err := watch.ParseExempt(params, []string{"handler", "args"})
	if err != nil {
		return nil, fmt.Errorf("Failed to parse watch (%#v): %v", params, err)
	}

	handler, hasHandler := wp.Exempt["handler"]
	if hasHandler {
		logger.Warn("The 'handler' field in watches has been deprecated " +
			"and replaced with the 'args' field. See https://www.consul.io/docs/agent/watches.html")
	}
	if _, ok := handler.(string); hasHandler && !ok {
		return nil, fmt.Errorf("Watch handler must be a string")
	}

	args, hasArgs := wp.Exempt["args"]
	if hasArgs {
		wp.Exempt["args"], err = parseWatchArgs(args)
		if err != nil {
			return nil, err
		}
	}

	if hasHandler && hasArgs || hasHandler && wp.HandlerType == "http" || hasArgs && wp.HandlerType == "http" {
		return nil, fmt.Errorf("Only one watch handler allowed")
	}
	if !hasHandler && !hasArgs && wp.HandlerType != "http" {
		return nil, fmt.Errorf("Must define a watch handler")
	}
	return wp, nil
}

func parseWatchArgs(args interface{}) ([]string, error) {
	switch args := args.(type) {
	case string:
		return []string{args}, nil
	case []string:
		return args, nil
	case []interface{}:
		result := make([]string, 0, len(args))
		for _, arg := range args {
			v, ok := arg.(string)
			if !ok {
				return nil, fmt.Errorf("Watch args must be a list of strings")
			}

			result = append(result, v)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("Watch args must be a list of strings")
	}
}
