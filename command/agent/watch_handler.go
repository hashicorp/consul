package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/armon/circbuf"
	consulapi "github.com/hashicorp/consul/api"
	consultemplate "github.com/marouenj/consul-template/core"
	"github.com/marouenj/consul/watch"
)

const (
	// Limit the size of a watch handlers's output to the
	// last WatchBufSize. Prevents an enormous buffer
	// from being captured
	WatchBufSize = 4 * 1024 // 4KB
	upAction     = "up"
	downAction   = "down"
)

// verifyWatchHandler does the pre-check for our handler configuration
func verifyWatchHandler(params interface{}) error {
	if params == nil {
		return fmt.Errorf("Must provide watch handler")
	}
	_, ok := params.(string)
	if !ok {
		return fmt.Errorf("Watch handler must be a string")
	}
	return nil
}

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

// makeWatchHandler returns a handler that would control a consul-template instance
func makeWatchHandlerForArchetype(logOutput io.Writer, agent *Agent, archetypeIndex int) watch.HandlerFunc {
	logger := log.New(logOutput, "", log.LstdFlags)

	// get the key
	archetype := agent.config.Archetypes[archetypeIndex]
	key := strings.Join([]string{"archetype", "watch", archetype.PoolName, archetype.ID}, "/")

	// http address
	address := []string{agent.config.Addresses.HTTP, strconv.Itoa(agent.config.Ports.HTTP)}

	// get the client
	client, _ := consulapi.NewClient(consulapi.DefaultConfig())

	// get the consul-template config
	config := consultemplate.DefaultConfig()
	config.Consul = strings.Join(address, ":")
	config.ConfigTemplates = append(config.ConfigTemplates, &agent.config.Archetypes[0].Template)

	// get the runner
	runner := agent.config.Runners[archetypeIndex]

	// define the selector
	var once sync.Once
	var selector func()

	fn := func(idx uint64, data interface{}) {
		logger.Printf("[INFO] agent: Calling watch handler on key %s", key)

		kv := client.KV()

		pair, _, err := kv.Get(key, nil)
		if err != nil {
			panic(err)
		}

		if pair == nil {
			return
		}
		val := string(pair.Value[:])

		actions := map[string]bool{
			upAction:   true,
			downAction: true,
		}

		if !actions[val] {
			return
		}

		// runner can be safely initialized
		// this bloc is executed once
		if runner == nil {
			logger.Printf("[INFO] agent: Initializing runner for key %s", key)
			if val != upAction {
				config.ConfigTemplates[0].First = true
			}
			runner, err = consultemplate.NewRunner(config, false, false)
			if err != nil {
				logger.Printf("[ERR] runner: Failed to launch: %v", err)
			}
			// define selector
			selector = func() {
			Outer:
				for {
					select {
					case err := <-runner.ErrCh:
						logger.Printf("%v", err)
					case <-runner.DoneCh:
						break Outer
					}
				}
			}

			if val == upAction {
				// runner.Up = true
				// runner.Init()
				go runner.Start()
			} else if val == downAction {
				go func() {
					once.Do(selector)
					once = sync.Once{}
				}()
			}
		}

		if val == upAction {
			go runner.Start()
			go func() {
				once.Do(selector)
				once = sync.Once{}
			}()
		} else if val == downAction {
			if err = runner.Stop(); err != nil {
				logger.Printf("%v", err)
			}
		}
	}
	return fn
}
