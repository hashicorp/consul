package debug

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/mitchellh/cli"
)

const (
	// debugInterval is the interval in which to capture dynamic information
	// when running debug
	debugInterval = 30 * time.Second

	// debugDuration is the total duration that debug runs before being
	// shut down
	debugDuration = 1 * time.Minute
)

func New(ui cli.Ui, shutdownCh <-chan struct{}) *cmd {
	ui = &cli.PrefixedUi{
		OutputPrefix: "==> ",
		InfoPrefix:   "    ",
		ErrorPrefix:  "==> ",
		Ui:           ui,
	}

	c := &cmd{UI: ui, shutdownCh: shutdownCh}
	c.init()
	return c
}

type cmd struct {
	UI    cli.Ui
	flags *flag.FlagSet
	http  *flags.HTTPFlags
	help  string

	shutdownCh <-chan struct{}

	// flags
	interval time.Duration
	duration time.Duration
	output   string
	archive  bool
	capture  []string
	client   *api.Client
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)

	defaultFilename := fmt.Sprintf("consul-debug-%d", time.Now().Unix())

	c.flags.Var((*flags.AppendSliceValue)(&c.capture), "capture",
		"One or more types of information to capture. This can be used "+
			"to capture a subset of information, and defaults to capturing "+
			"everything available. Possible information for capture: "+
			"This can be repeated multiple times.")
	c.flags.DurationVar(&c.interval, "interval", debugInterval,
		fmt.Sprintf("The interval in which to capture dynamic information such as "+
			"telemetry, and profiling. Defaults to %s.", debugInterval))
	c.flags.DurationVar(&c.duration, "duration", debugDuration,
		fmt.Sprintf("The total time to record information. "+
			"Defaults to %s.", debugDuration))
	c.flags.BoolVar(&c.archive, "archive", true, "Boolean value for if the files "+
		"should be archived and compressed. Setting this to false will skip the "+
		"archive step and leave the directory of information on the relative path.")
	c.flags.StringVar(&c.output, "output", defaultFilename, "The path "+
		"to the compressed archive that will be created with the "+
		"information after collection.")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	// TODO do we need server flags?
	flags.Merge(c.flags, c.http.ServerFlags())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		c.UI.Error(fmt.Sprintf("Error parsing flags: %s", err))
		return 1
	}

	// Connect to the agent
	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}
	c.client = client

	// Retrieve and process agent information necessary to validate
	self, err := client.Agent().Self()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error querying target agent: %s. Verify connectivity and agent address.", err))
		return 1
	}

	version, ok := self["Config"]["Version"].(string)
	if !ok {
		c.UI.Error(fmt.Sprintf("Agent response did not contain version key: %v", self))
		return 1
	}

	debugEnabled, ok := self["DebugConfig"]["EnableDebug"].(bool)
	if !ok {
		c.UI.Error(fmt.Sprintf("Agent response did not contain debug key: %v", self))
		return 1
	}

	err = c.prepare(debugEnabled)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Capture validation failed: %v", err))
		return 1
	}

	c.UI.Output("Starting debugger and capturing static information...")

	// Output metadata about target agent
	c.UI.Info(fmt.Sprintf(" Agent Address: '%s'", "something"))
	c.UI.Info(fmt.Sprintf(" Agent Version: '%s'", version))
	c.UI.Info(fmt.Sprintf("      Interval: '%s'", c.interval))
	c.UI.Info(fmt.Sprintf("      Duration: '%s'", c.duration))
	c.UI.Info(fmt.Sprintf("        Output: '%s'", c.output))
	c.UI.Info(fmt.Sprintf("       Capture: '%s'", strings.Join(c.capture, ", ")))

	// Capture static information from the target agent
	err = c.captureStatic()
	if err != nil {
		c.UI.Warn(fmt.Sprintf("Static capture failed: %v", err))
	}

	c.UI.Info(fmt.Sprintf("Saved debug archive: %s", c.output))

	// Block and monitor for duration
	return 0
}

// prepare validates agent settings against targets and prepares the environment for capturing
func (c *cmd) prepare(debugEnabled bool) error {
	// If none are specified we will collect information from
	// all by default
	if len(c.capture) == 0 {
		c.capture = c.defaultTargets()
	}

	if !debugEnabled && c.configuredTarget("pprof") {
		cs := c.capture
		for i := 0; i < len(cs); i++ {
			if cs[i] == "pprof" {
				c.capture = append(cs[:i], cs[i+1:]...)
				i--
			}
		}
		c.UI.Warn("[WARN] Unable to capture pprof. Set enable_debug to true on target agent to enable profiling.")
	}

	for _, t := range c.capture {
		if !c.allowedTarget(t) {
			return fmt.Errorf("target not found: %s", t)
		}
	}

	if _, err := os.Stat(c.output); os.IsNotExist(err) {
		err := os.MkdirAll(c.output, 0755)
		if err != nil {
			return fmt.Errorf("could not create output directory: %s", err)
		}
	} else {
		return fmt.Errorf("output directory already exists: %s", c.output)
	}

	return nil
}

// captureStatic captures static target information and writes it
// to the output path
func (c *cmd) captureStatic() error {
	// Collect errors via multierror as we want to gracefully
	// fail if an API is inacessible
	var errors error

	// Collect the named outputs here
	outputs := make(map[string]interface{}, 0)

	// Capture host information
	if c.configuredTarget("host") {
		host, err := c.client.Agent().Host()
		if err != nil {
			errors = multierror.Append(errors, err)
		}
		outputs["host"] = host
	}

	// Capture agent information
	if c.configuredTarget("agent") {
		agent, err := c.client.Agent().Self()
		if err != nil {
			errors = multierror.Append(errors, err)
		}
		outputs["agent"] = agent
	}

	// Capture cluster members information, including WAN
	if c.configuredTarget("cluster") {
		members, err := c.client.Agent().Members(true)
		if err != nil {
			errors = multierror.Append(errors, err)
		}
		outputs["members"] = members
	}

	// Write all outputs to disk
	for output, v := range outputs {
		marshaled, err := json.MarshalIndent(v, "", "\t")
		if err != nil {
			errors = multierror.Append(errors, err)
		}

		err = ioutil.WriteFile(fmt.Sprintf("%s/%s.json", c.output, output), marshaled, 0644)
		if err != nil {
			errors = multierror.Append(errors, err)
		}
	}

	return errors
}

// allowedTarget returns a boolean if the target is able to be captured
func (c *cmd) allowedTarget(target string) bool {
	for _, dt := range c.defaultTargets() {
		if dt == target {
			return true
		}
	}
	return false
}

// configuredTarget returns a boolean if the target is configured to be
// captured in the command
func (c *cmd) configuredTarget(target string) bool {
	for _, dt := range c.capture {
		if dt == target {
			return true
		}
	}
	return false
}

func (c *cmd) defaultTargets() []string {
	return []string{"metrics", "pprof", "logs", "host", "agent", "cluster"}
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Monitors a Consul agent for the specified period of time, recording information about the agent, cluster, and environment to an archive written to the relative directory."
const help = `
Usage: consul debug [options]

  Monitors a Consul agent for the specified period of time, recording
  information about the agent, cluster, and environment to an archive
  written to the relative directory.
`
