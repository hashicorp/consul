package command

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/armon/consul-api"
	"github.com/mitchellh/cli"
)

const (
	// rExecPrefix is the prefix in the KV store used to
	// store the remote exec data
	rExecPrefix = "_rexec"

	// rExecFileName is the name of the file we append to
	// the path, e.g. _rexec/session_id/job
	rExecFileName = "job"

	// rExecAck is the suffix added to an ack path
	rExecAckSuffix = "/ack"

	// rExecAck is the suffix added to an exit code
	rExecExitSuffix = "/exit"

	// rExecOutputDivider is used to namespace the output
	rExecOutputDivider = "/out/"
)

// rExecConf is used to pass around configuration
type rExecConf struct {
	datacenter string
	prefix     string

	node    string
	service string
	tag     string

	wait     time.Duration
	replWait time.Duration

	cmd    string
	script []byte

	verbose bool
}

// rExecEvent is the event we broadcast using a user-event
type rExecEvent struct {
	Prefix  string
	Session string
}

// rExecSpec is the file we upload to specify the parameters
// of the remote execution.
type rExecSpec struct {
	// Command is a single command to run directly in the shell
	Command string `json:",omitempty"`

	// Script should be spilled to a file and executed
	Script []byte `json:",omitempty"`

	// Wait is how long we are waiting on a quiet period to terminate
	Wait time.Duration
}

// rExecAck is used to transmit an acknowledgement
type rExecAck struct {
	Node string
}

// rExecHeart is used to transmit a heartbeat
type rExecHeart struct {
	Node string
}

// rExecOutput is used to transmit a chunk of output
type rExecOutput struct {
	Node   string
	Output []byte
}

// rExecExit is used to transmit an exit code
type rExecExit struct {
	Node string
	Code int
}

// ExecCommand is a Command implementation that is used to
// do remote execution of commands
type ExecCommand struct {
	ShutdownCh <-chan struct{}
	Ui         cli.Ui
	conf       rExecConf
	client     *consulapi.Client
	sessionID  string
}

func (c *ExecCommand) Run(args []string) int {
	cmdFlags := flag.NewFlagSet("exec", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }
	cmdFlags.StringVar(&c.conf.datacenter, "datacenter", "", "")
	cmdFlags.StringVar(&c.conf.node, "node", "", "")
	cmdFlags.StringVar(&c.conf.service, "service", "", "")
	cmdFlags.StringVar(&c.conf.tag, "tag", "", "")
	cmdFlags.StringVar(&c.conf.prefix, "prefix", rExecPrefix, "")
	cmdFlags.DurationVar(&c.conf.replWait, "wait-repl", 100*time.Millisecond, "")
	cmdFlags.DurationVar(&c.conf.wait, "wait", time.Second, "")
	cmdFlags.BoolVar(&c.conf.verbose, "verbose", false, "")
	httpAddr := HTTPAddrFlag(cmdFlags)
	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	// Join the commands to execute
	c.conf.cmd = strings.Join(cmdFlags.Args(), " ")

	// If there is no command, read stdin for a script input
	if c.conf.cmd == "-" {
		c.conf.cmd = ""
		var buf bytes.Buffer
		_, err := io.Copy(&buf, os.Stdin)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to read stdin: %v", err))
			c.Ui.Error("")
			c.Ui.Error(c.Help())
			return 1
		}
		c.conf.script = buf.Bytes()
	}

	// Ensure we have a command or script
	if c.conf.cmd == "" && len(c.conf.script) == 0 {
		c.Ui.Error("Must specify a command to execute")
		c.Ui.Error("")
		c.Ui.Error(c.Help())
		return 1
	}

	// Validate the configuration
	if err := c.conf.validate(); err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	// Create and test the HTTP client
	client, err := HTTPClientDC(*httpAddr, c.conf.datacenter)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}
	_, err = client.Agent().NodeName()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying Consul agent: %s", err))
		return 1
	}
	c.client = client

	// Create the job spec
	spec, err := c.makeRExecSpec()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to create job spec: %s", err))
		return 1
	}

	// Create a session for this
	c.sessionID, err = c.createSession()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to create session: %s", err))
		return 1
	}
	defer c.destroySession()

	// Upload the payload
	if err := c.uploadPayload(spec); err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to create job file: %s", err))
		return 1
	}
	defer c.destroyData()

	// Wait for replication. This is done so that when the event is
	// received, the job file can be read using a stale read. If the
	// stale read fails, we expect a consistent read to be done, so
	// largely this is a heuristic.
	select {
	case <-time.After(c.conf.replWait):
	case <-c.ShutdownCh:
		return 1
	}

	// Fire the event
	id, err := c.fireEvent()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to fire event: %s", err))
		return 1
	}
	if c.conf.verbose {
		c.Ui.Output(fmt.Sprintf("Fired remote execution event. ID: %s", id))
	}

	// Wait for the job to finish now
	return c.waitForJob()
}

// waitForJob is used to poll for results and wait until the job is terminated
func (c *ExecCommand) waitForJob() int {
	// Although the session destroy is already deferred, we do it again here,
	// because invalidation of the session before destroyData() ensures there is
	// no race condition allowing an agent to upload data (the acquire will fail).
	defer c.destroySession()
	start := time.Now()
	ackCh := make(chan rExecAck, 128)
	heartCh := make(chan rExecHeart, 128)
	outputCh := make(chan rExecOutput, 128)
	exitCh := make(chan rExecExit, 128)
	doneCh := make(chan struct{})
	errCh := make(chan struct{}, 1)
	defer close(doneCh)
	go c.streamResults(doneCh, ackCh, heartCh, outputCh, exitCh, errCh)
	var ackCount, exitCount int
OUTER:
	for {
		// Determine wait time. We provide a larger window if we know about
		// nodes which are still working.
		waitIntv := c.conf.wait
		if ackCount > exitCount {
			waitIntv *= 4
		}

		select {
		case e := <-ackCh:
			ackCount++
			c.Ui.Output(fmt.Sprintf("Node %s: acknowledged event", e.Node))

		case h := <-heartCh:
			if c.conf.verbose {
				c.Ui.Output(fmt.Sprintf("Node %s: heartbeated", h.Node))
			}

		case e := <-outputCh:
			c.Ui.Output(fmt.Sprintf("Node %s: %s", e.Node, e.Output))

		case e := <-exitCh:
			exitCount++
			c.Ui.Output(fmt.Sprintf("Node %s: exited with code %d", e.Node, e.Code))

		case <-time.After(waitIntv):
			c.Ui.Output(fmt.Sprintf("%d / %d node(s) completed / acknowledged", exitCount, ackCount))
			c.Ui.Output(fmt.Sprintf("Exec complete in %0.2f seconds",
				float64(time.Now().Sub(start))/float64(time.Second)))
			break OUTER

		case <-errCh:
			return 1

		case <-c.ShutdownCh:
			return 1
		}
	}
	return 0
}

// streamResults is used to perform blocking queries against the KV endpoint and stream in
// notice of various events into waitForJob
func (c *ExecCommand) streamResults(doneCh chan struct{}, ackCh chan rExecAck, heartCh chan rExecHeart,
	outputCh chan rExecOutput, exitCh chan rExecExit, errCh chan struct{}) {
	kv := c.client.KV()
	opts := consulapi.QueryOptions{WaitTime: c.conf.wait}
	dir := path.Join(c.conf.prefix, c.sessionID) + "/"
	seen := make(map[string]struct{})

	for {
		// Check if we've been signaled to exit
		select {
		case <-doneCh:
			return
		default:
		}

		// Block on waiting for new keys
		keys, qm, err := kv.Keys(dir, "", &opts)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to read results: %s", err))
			goto ERR_EXIT
		}

		// Fast-path the no-change case
		if qm.LastIndex == opts.WaitIndex {
			continue
		}
		opts.WaitIndex = qm.LastIndex

		// Handle each key
		for _, key := range keys {
			// Ignore if we've seen it
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}

			// Trim the directory
			full := key
			key = strings.TrimPrefix(key, dir)

			// Handle the key type
			switch {
			case key == rExecFileName:
				continue
			case strings.HasSuffix(key, rExecAckSuffix):
				ackCh <- rExecAck{Node: strings.TrimSuffix(key, rExecAckSuffix)}

			case strings.HasSuffix(key, rExecExitSuffix):
				pair, _, err := kv.Get(full, nil)
				if err != nil {
					c.Ui.Error(fmt.Sprintf("Failed to read key '%s': %v", full, err))
					continue
				}
				code, err := strconv.ParseInt(string(pair.Value), 10, 32)
				if err != nil {
					c.Ui.Error(fmt.Sprintf("Failed to parse exit code '%s': %v", pair.Value, err))
					continue
				}
				exitCh <- rExecExit{
					Node: strings.TrimSuffix(key, rExecExitSuffix),
					Code: int(code),
				}

			case strings.LastIndex(key, rExecOutputDivider) != -1:
				pair, _, err := kv.Get(full, nil)
				if err != nil {
					c.Ui.Error(fmt.Sprintf("Failed to read key '%s': %v", full, err))
					continue
				}
				idx := strings.LastIndex(key, rExecOutputDivider)
				node := key[:idx]
				if len(pair.Value) == 0 {
					heartCh <- rExecHeart{Node: node}
				} else {
					outputCh <- rExecOutput{Node: node, Output: pair.Value}
				}

			default:
				c.Ui.Error(fmt.Sprintf("Unknown key '%s', ignoring.", key))
			}
		}
	}
	return

ERR_EXIT:
	select {
	case errCh <- struct{}{}:
	default:
	}
}

// validate checks that the configuration is sane
func (conf *rExecConf) validate() error {
	// Validate the filters
	if conf.node != "" {
		if _, err := regexp.Compile(conf.node); err != nil {
			return fmt.Errorf("Failed to compile node filter regexp: %v", err)
		}
	}
	if conf.service != "" {
		if _, err := regexp.Compile(conf.service); err != nil {
			return fmt.Errorf("Failed to compile service filter regexp: %v", err)
		}
	}
	if conf.tag != "" {
		if _, err := regexp.Compile(conf.tag); err != nil {
			return fmt.Errorf("Failed to compile tag filter regexp: %v", err)
		}
	}
	if conf.tag != "" && conf.service == "" {
		return fmt.Errorf("Cannot provide tag filter without service filter.")
	}
	return nil
}

// createSession is used to create a new session for this command
func (c *ExecCommand) createSession() (string, error) {
	session := c.client.Session()
	se := consulapi.SessionEntry{
		Name: "Remote Exec",
	}
	id, _, err := session.Create(&se, nil)
	return id, err
}

// destroySession is used to destroy the associated session
func (c *ExecCommand) destroySession() error {
	session := c.client.Session()
	_, err := session.Destroy(c.sessionID, nil)
	return err
}

// makeRExecSpec creates a serialized job specification
// that can be uploaded which will be parsed by agents to
// determine what to do.
func (c *ExecCommand) makeRExecSpec() ([]byte, error) {
	spec := &rExecSpec{
		Command: c.conf.cmd,
		Script:  c.conf.script,
		Wait:    c.conf.wait,
	}
	return json.Marshal(spec)
}

// uploadPayload is used to upload the request payload
func (c *ExecCommand) uploadPayload(payload []byte) error {
	kv := c.client.KV()
	pair := consulapi.KVPair{
		Key:     path.Join(c.conf.prefix, c.sessionID, rExecFileName),
		Value:   payload,
		Session: c.sessionID,
	}
	ok, _, err := kv.Acquire(&pair, nil)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("failed to acquire key %s", pair.Key)
	}
	return nil
}

// destroyData is used to nuke all the data associated with
// this remote exec. We just do a recursive delete of our
// data directory.
func (c *ExecCommand) destroyData() error {
	kv := c.client.KV()
	dir := path.Join(c.conf.prefix, c.sessionID)
	_, err := kv.DeleteTree(dir, nil)
	return err
}

// fireEvent is used to fire the event that will notify nodes
// about the remote execution. Returns the event ID or error
func (c *ExecCommand) fireEvent() (string, error) {
	// Create the user event payload
	msg := &rExecEvent{
		Prefix:  c.conf.prefix,
		Session: c.sessionID,
	}
	buf, err := json.Marshal(msg)
	if err != nil {
		return "", err
	}

	// Format the user event
	event := c.client.Event()
	params := &consulapi.UserEvent{
		Name:          "_rexec",
		Payload:       buf,
		NodeFilter:    c.conf.node,
		ServiceFilter: c.conf.service,
		TagFilter:     c.conf.tag,
	}

	// Fire the event
	id, _, err := event.Fire(params, nil)
	return id, err
}

func (c *ExecCommand) Synopsis() string {
	return "Executes a command on Consul nodes"
}

func (c *ExecCommand) Help() string {
	helpText := `
Usage: consul exec [options] [-|command...]

  Evaluates a command on remote Consul nodes. The nodes responding can
  be filtered using regular expressions on node name, service, and tag
  definitions. If a command is '-', stdin will be read until EOF
  and used as a script input.

Options:

  -http-addr=127.0.0.1:8500  HTTP address of the Consul agent.
  -datacenter=""             Datacenter to dispatch in. Defaults to that of agent.
  -prefix="_rexec"           Prefix in the KV store to use for request data
  -node=""					 Regular expression to filter on node names
  -service=""                Regular expression to filter on service instances
  -tag=""                    Regular expression to filter on service tags. Must be used
                             with -service.
  -wait=1s                   Period to wait with no responses before terminating execution.
  -wait-repl=100ms           Period to wait for replication before firing event. This is an
                             optimization to allow stale reads to be performed.
  -verbose                   Enables verbose output
`
	return strings.TrimSpace(helpText)
}
