// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package watch

import (
	"context"
	"fmt"
	"io"
	"log"
	"reflect"
	"time"

	"github.com/hashicorp/go-hclog"

	consulapi "github.com/hashicorp/consul/api"
)

const (
	// retryInterval is the base retry value
	retryInterval = 5 * time.Second

	// maximum back off time, this is to prevent
	// exponential runaway
	maxBackoffTime = 180 * time.Second

	// Name used with hclog Logger. We do not add this to the logging package
	// because we do not want to pull in the root consul module.
	watchLoggerName = "watch"
)

func (p *Plan) Run(address string) error {
	return p.RunWithConfig(address, nil)
}

// Run is used to run a watch plan
func (p *Plan) RunWithConfig(address string, conf *consulapi.Config) error {
	logger := p.Logger
	if logger == nil {
		logger = newWatchLogger(p.LogOutput)
	}

	// Setup the client
	p.address = address
	if conf == nil {
		conf = consulapi.DefaultConfigWithLogger(logger)
	}
	conf.Address = address
	conf.Datacenter = p.Datacenter
	conf.Token = p.Token
	client, err := consulapi.NewClient(conf)
	if err != nil {
		return fmt.Errorf("Failed to connect to agent: %v", err)
	}

	if p.AllDatacenters {
		return p.RunWithClientAndHclogAllDatacenters(conf, client, logger)
	} else {
		return p.RunWithClientAndHclog(client, logger)
	}
}

// RunWithClientAndLogger runs a watch plan using an external client and
// hclog.Logger instance. Using this, the plan's Datacenter, Token and LogOutput
// fields are ignored and the passed client is expected to be configured as
// needed.
func (p *Plan) RunWithClientAndHclog(client *consulapi.Client, logger hclog.Logger) error {
	var watchLogger hclog.Logger
	if logger == nil {
		watchLogger = newWatchLogger(nil)
	} else {
		watchLogger = logger.Named(watchLoggerName)
	}

	p.client = client

	// Loop until we are canceled
	failures := 0
OUTER:
	for !p.shouldStop() {
		// Invoke the handler
		blockParamVal, result, err := p.Watcher(p)

		// Check if we should terminate since the function
		// could have blocked for a while
		if p.shouldStop() {
			break
		}

		// Handle an error in the watch function
		if err != nil {
			// Perform an exponential backoff
			failures++
			if blockParamVal == nil {
				p.lastParamVal = nil
			} else {
				p.lastParamVal = blockParamVal.Next(p.lastParamVal)
			}
			retry := retryInterval * time.Duration(failures*failures)
			if retry > maxBackoffTime {
				retry = maxBackoffTime
			}
			watchLogger.Error("Watch errored", "type", p.Type, "error", err, "retry", retry)
			select {
			case <-time.After(retry):
				continue OUTER
			case <-p.stopCh:
				return nil
			}
		}

		// Clear the failures
		failures = 0

		// If the index is unchanged do nothing
		if p.lastParamVal != nil && p.lastParamVal.Equal(blockParamVal) {
			continue
		}

		// Update the index, look for change
		oldParamVal := p.lastParamVal
		p.lastParamVal = blockParamVal.Next(oldParamVal)
		if oldParamVal != nil && reflect.DeepEqual(p.lastResult, result) {
			continue
		}

		// Handle the updated result
		p.lastResult = result
		// If a hybrid handler exists use that
		if p.HybridHandler != nil {
			p.HybridHandler(blockParamVal, result)
		} else if p.Handler != nil {
			idx, ok := blockParamVal.(WaitIndexVal)
			if !ok {
				watchLogger.Error("Handler only supports index-based " +
					" watches but non index-based watch run. Skipping Handler.")
			}
			p.Handler(uint64(idx), result)
		}
	}
	return nil
}

// RunWithClientAndHclogAllDatacenters runs a watch plan using an external client and
// hclog.Logger instance. The plan queries on all datacenters.
func (p *Plan) RunWithClientAndHclogAllDatacenters(conf *consulapi.Config, client *consulapi.Client, logger hclog.Logger) error {
	var watchLogger hclog.Logger
	if logger == nil {
		watchLogger = newWatchLogger(nil)
	} else {
		watchLogger = logger.Named(watchLoggerName)
	}

	// Loop until we are canceled
	failures := 0

	p.mapLastParamVal = make(map[string]BlockingParamVal)
	p.mapLastResult = make(map[string]interface{})
OUTER:
	for !p.shouldStop() {
		p.client = client
		blockParamVal := make(map[string]BlockingParamVal)
		result := make(map[string]interface{})
		catalog := p.client.Catalog()
		dcs, err := catalog.Datacenters()
		if err != nil || len(dcs) == 0 {
			dcs = append(dcs, "") //This will cause to use default DataCenter if err
		}

		for _, dc := range dcs {
			p.Datacenter = dc
			conf.Address = p.address
			conf.Datacenter = dc
			conf.Token = p.Token
			var clientDC *consulapi.Client
			clientDC, err = consulapi.NewClient(conf)
			if err != nil {
				return fmt.Errorf("Failed to connect to agent: %v", err)
			}
			p.client = clientDC

			// Invoke the handler
			blockParamVal[dc], result[dc], err = p.Watcher(p)

			// Check if we should terminate since the function
			// could have blocked for a while
			if p.shouldStop() {
				break OUTER
			}

			// Handle an error in the watch function
			if err != nil {
				// Perform an exponential backoff
				failures++
				if blockParamVal[dc] == nil {
					p.mapLastParamVal[dc] = nil
				} else {
					p.mapLastParamVal[dc] = blockParamVal[dc].Next(p.mapLastParamVal[dc])
				}
				retry := retryInterval * time.Duration(failures*failures)
				if retry > maxBackoffTime {
					retry = maxBackoffTime
				}
				watchLogger.Error("Watch errored", "type", p.Type, "error", err, "retry", retry)
				select {
				case <-time.After(retry):
					continue OUTER
				case <-p.stopCh:
					return nil
				}
			}
		}

		// Clear the failures
		failures = 0

		// If the index is unchanged do nothing
		noChange := true
		for dc, lastParamVal := range p.mapLastParamVal {
			if lastParamVal != nil && !lastParamVal.Equal(blockParamVal[dc]) {
				noChange = false
				break
			}
		}
		if noChange && len(p.mapLastParamVal) > 0 {
			continue OUTER
		}

		// Update the index, look for change
		noChange = true
		for _, dc := range dcs {
			oldParamVal, ok := p.mapLastParamVal[dc]
			if !ok {
				oldParamVal = nil
			}
			p.mapLastParamVal[dc] = blockParamVal[dc].Next(oldParamVal)
			if oldParamVal != nil {
				if !reflect.DeepEqual(p.mapLastResult[dc], result[dc]) {
					noChange = false
				}
			} else {
				noChange = false
			}
		}
		if noChange {
			continue OUTER
		}

		// Handle the updated result
		var totResult []interface{}
		for dc, rdc := range result {
			p.mapLastResult[dc] = rdc
			if rdc != nil {
				totResult = append(totResult, rdc)
			}
		}

		// If a hybrid handler exists use that
		if p.HybridHandler != nil {
			p.HybridHandler(blockParamVal[dcs[0]], totResult)
		} else if p.Handler != nil {
			idx, ok := blockParamVal[dcs[0]].(WaitIndexVal)
			if !ok {
				watchLogger.Error("Handler only supports index-based " +
					" watches but non index-based watch run. Skipping Handler.")
			}
			p.Handler(uint64(idx), totResult)
		}
	}
	return nil
}

// Deprecated: Use RunwithClientAndHclog
func (p *Plan) RunWithClientAndLogger(client *consulapi.Client, logger *log.Logger) error {

	p.client = client

	// Loop until we are canceled
	failures := 0
OUTER:
	for !p.shouldStop() {
		// Invoke the handler
		blockParamVal, result, err := p.Watcher(p)

		// Check if we should terminate since the function
		// could have blocked for a while
		if p.shouldStop() {
			break
		}

		// Handle an error in the watch function
		if err != nil {
			// Perform an exponential backoff
			failures++
			if blockParamVal == nil {
				p.lastParamVal = nil
			} else {
				p.lastParamVal = blockParamVal.Next(p.lastParamVal)
			}
			retry := retryInterval * time.Duration(failures*failures)
			if retry > maxBackoffTime {
				retry = maxBackoffTime
			}
			logger.Printf("[ERR] consul.watch: Watch (type: %s) errored: %v, retry in %v",
				p.Type, err, retry)
			select {
			case <-time.After(retry):
				continue OUTER
			case <-p.stopCh:
				return nil
			}
		}

		// Clear the failures
		failures = 0

		// If the index is unchanged do nothing
		if p.lastParamVal != nil && p.lastParamVal.Equal(blockParamVal) {
			continue
		}

		// Update the index, look for change
		oldParamVal := p.lastParamVal
		p.lastParamVal = blockParamVal.Next(oldParamVal)
		if oldParamVal != nil && reflect.DeepEqual(p.lastResult, result) {
			continue
		}

		// Handle the updated result
		p.lastResult = result
		// If a hybrid handler exists use that
		if p.HybridHandler != nil {
			p.HybridHandler(blockParamVal, result)
		} else if p.Handler != nil {
			idx, ok := blockParamVal.(WaitIndexVal)
			if !ok {
				logger.Printf("[ERR] consul.watch: Handler only supports index-based " +
					" watches but non index-based watch run. Skipping Handler.")
			}
			p.Handler(uint64(idx), result)
		}
	}
	return nil
}

// Stop is used to stop running the watch plan
func (p *Plan) Stop() {
	p.stopLock.Lock()
	defer p.stopLock.Unlock()
	if p.stop {
		return
	}
	p.stop = true
	if p.cancelFunc != nil {
		p.cancelFunc()
	}
	close(p.stopCh)
}

func (p *Plan) shouldStop() bool {
	select {
	case <-p.stopCh:
		return true
	default:
		return false
	}
}

func (p *Plan) setCancelFunc(cancel context.CancelFunc) {
	p.stopLock.Lock()
	defer p.stopLock.Unlock()
	if p.shouldStop() {
		// The watch is stopped and execute the new cancel func to stop watchFactory
		cancel()
		return
	}
	p.cancelFunc = cancel
}

func (p *Plan) IsStopped() bool {
	p.stopLock.Lock()
	defer p.stopLock.Unlock()
	return p.stop
}

func newWatchLogger(output io.Writer) hclog.Logger {
	return hclog.New(&hclog.LoggerOptions{
		Name:   watchLoggerName,
		Output: output,
	})
}
