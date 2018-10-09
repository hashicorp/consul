package debug

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	debugDuration = 2 * time.Minute

	// debugDurationGrace is a period of time added to the specified
	// duration to allow intervals to capture within that time
	debugDurationGrace = 2 * time.Second

	// debugMinInterval is the minimum a user can configure the interval
	// to prevent accidental DOS
	debugMinInterval = 5 * time.Second

	// debugMinDuration is the minimum a user can configure the duration
	// to ensure that all information can be collected in time
	debugMinDuration = 10 * time.Second

	// The extension for archive files
	debugArchiveExtension = ".tar.gz"
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
	// validateTiming can be used to skip validation of interval, duration. This
	// is primarily useful for testing
	validateTiming bool
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)

	defaultFilename := fmt.Sprintf("consul-debug-%d", time.Now().Unix())

	c.flags.Var((*flags.AppendSliceValue)(&c.capture), "capture",
		fmt.Sprintf("One or more types of information to capture. This can be used "+
			"to capture a subset of information, and defaults to capturing "+
			"everything available. Possible information for capture: %s. "+
			"This can be repeated multiple times.", strings.Join(c.defaultTargets(), ", ")))
	c.flags.DurationVar(&c.interval, "interval", debugInterval,
		fmt.Sprintf("The interval in which to capture dynamic information such as "+
			"telemetry, and profiling. Defaults to %s.", debugInterval))
	c.flags.DurationVar(&c.duration, "duration", debugDuration,
		fmt.Sprintf("The total time to record information. "+
			"Defaults to %s.", debugDuration))
	c.flags.BoolVar(&c.archive, "archive", true, "Boolean value for if the files "+
		"should be archived and compressed. Setting this to false will skip the "+
		"archive step and leave the directory of information on the current path.")
	c.flags.StringVar(&c.output, "output", defaultFilename, "The path "+
		"to the compressed archive that will be created with the "+
		"information after collection.")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	// TODO do we need server flags?
	flags.Merge(c.flags, c.http.ServerFlags())
	c.help = flags.Usage(help, c.flags)

	c.validateTiming = true
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		c.UI.Error(fmt.Sprintf("Error parsing flags: %s", err))
		return 1
	}

	if len(c.flags.Args()) > 0 {
		c.UI.Error("debug: Too many arguments provided, expected 0")
		return 1
	}

	// Connect to the agent
	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connecting to Consul agent: %s", err))
		return 1
	}
	c.client = client

	version, err := c.prepare()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Capture validation failed: %v", err))
		return 1
	}

	archiveName := c.output
	// Show the user the final file path if archiving
	if c.archive {
		archiveName = archiveName + debugArchiveExtension
	}

	c.UI.Output("Starting debugger and capturing static information...")

	// Output metadata about target agent
	c.UI.Info(fmt.Sprintf(" Agent Address: '%s'", "TODO"))
	c.UI.Info(fmt.Sprintf(" Agent Version: '%s'", version))
	c.UI.Info(fmt.Sprintf("      Interval: '%s'", c.interval))
	c.UI.Info(fmt.Sprintf("      Duration: '%s'", c.duration))
	c.UI.Info(fmt.Sprintf("        Output: '%s'", archiveName))
	c.UI.Info(fmt.Sprintf("       Capture: '%s'", strings.Join(c.capture, ", ")))

	// Add the extra grace period to ensure
	// all intervals will be captured within the time allotted
	c.duration = c.duration + debugDurationGrace

	// Capture static information from the target agent
	err = c.captureStatic()
	if err != nil {
		c.UI.Warn(fmt.Sprintf("Static capture failed: %v", err))
	}

	// Capture dynamic information from the target agent, blocking for duration
	// TODO(pearkes): figure out a cleaner way to do this
	if c.configuredTarget("metrics") || c.configuredTarget("logs") || c.configuredTarget("pprof") {
		err = c.captureDynamic()
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error encountered during collection: %v", err))
		}
	}

	// Archive the data if configured to
	if c.archive {
		err = c.createArchive()

		if err != nil {
			c.UI.Warn(fmt.Sprintf("Archive creation failed: %v", err))
			return 1
		}
	}

	c.UI.Info(fmt.Sprintf("Saved debug archive: %s", archiveName))

	return 0
}

// prepare validates agent settings against targets and prepares the environment for capturing
func (c *cmd) prepare() (version string, err error) {
	// Ensure realistic duration and intervals exists
	if c.validateTiming {
		if c.duration < debugMinDuration {
			return "", fmt.Errorf("duration must be longer than %s", debugMinDuration)
		}

		if c.interval < debugMinInterval {
			return "", fmt.Errorf("interval must be longer than %s", debugMinDuration)
		}

		if c.duration < c.interval {
			return "", fmt.Errorf("duration (%s) must be longer than interval (%s)", c.duration, c.interval)
		}
	}

	// Retrieve and process agent information necessary to validate
	self, err := c.client.Agent().Self()
	if err != nil {
		return "", fmt.Errorf("error querying target agent: %s. verify connectivity and agent address", err)
	}

	version, ok := self["Config"]["Version"].(string)
	if !ok {
		return "", fmt.Errorf("agent response did not contain version key")
	}

	debugEnabled, ok := self["DebugConfig"]["EnableDebug"].(bool)
	if !ok {
		return version, fmt.Errorf("agent response did not contain debug key")
	}

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
			return version, fmt.Errorf("target not found: %s", t)
		}
	}

	if _, err := os.Stat(c.output); os.IsNotExist(err) {
		err := os.MkdirAll(c.output, 0755)
		if err != nil {
			return version, fmt.Errorf("could not create output directory: %s", err)
		}
	} else {
		return version, fmt.Errorf("output directory already exists: %s", c.output)
	}

	return version, nil
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

	// Write all outputs to disk as JSON
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

// captureDynamic blocks for the duration of the command
// specified by the duration flag, capturing the dynamic
// targets at the interval specified
func (c *cmd) captureDynamic() error {
	successChan := make(chan int64)
	errCh := make(chan error)
	durationChn := time.After(c.duration)
	intervalCount := 0

	c.UI.Output(fmt.Sprintf("Beginning capture interval %s (%d)", time.Now().Local().String(), intervalCount))

	// We'll wait for all of the targets configured to be
	// captured before continuing
	var wg sync.WaitGroup

	capture := func() {
		timestamp := time.Now().Local().Unix()

		// Make the directory that will store all captured data
		// for this interval
		timestampDir := fmt.Sprintf("%s/%d", c.output, timestamp)
		err := os.MkdirAll(timestampDir, 0755)
		if err != nil {
			errCh <- err
		}

		// Capture metrics
		if c.configuredTarget("metrics") {
			wg.Add(1)

			go func() {
				metrics, err := c.client.Agent().Metrics()
				if err != nil {
					errCh <- err
				}

				marshaled, err := json.MarshalIndent(metrics, "", "\t")
				if err != nil {
					errCh <- err
				}

				err = ioutil.WriteFile(fmt.Sprintf("%s/%s.json", timestampDir, "metrics"), marshaled, 0755)
				if err != nil {
					errCh <- err
				}

				// Sleep as other dynamic targets wait collect for the whole interv	al
				time.Sleep(c.interval)

				wg.Done()
			}()
		}

		// Capture pprof
		if c.configuredTarget("pprof") {
			wg.Add(1)

			go func() {
				pprofOutputs := make(map[string][]byte, 0)

				heap, err := c.client.Debug().Heap()
				if err != nil {
					errCh <- err
				}
				pprofOutputs["heap"] = heap

				// Capture a profile with a minimum of 1s
				// TODO should be min across the board
				s := c.interval.Seconds()
				if s < 1 {
					s = 1
				}

				// This will block for the interval
				prof, err := c.client.Debug().Profile(int(s))
				if err != nil {
					errCh <- err
				}
				pprofOutputs["profile"] = prof

				gr, err := c.client.Debug().Goroutine()
				if err != nil {
					errCh <- err
				}
				pprofOutputs["goroutine"] = gr

				// Write profiles to disk
				for output, v := range pprofOutputs {
					err = ioutil.WriteFile(fmt.Sprintf("%s/%s.prof", timestampDir, output), v, 0755)
					if err != nil {
						errCh <- err
					}
				}

				wg.Done()
			}()
		}

		// Capture logs
		if c.configuredTarget("logs") {
			wg.Add(1)

			go func() {
				endLogChn := make(chan struct{})
				logCh, err := c.client.Agent().Monitor("DEBUG", endLogChn, nil)
				if err != nil {
					errCh <- err
				}
				// Close the log stream
				defer close(endLogChn)

				// Create the log file for writing
				f, err := os.Create(fmt.Sprintf("%s/%s", timestampDir, "consul.log"))
				if err != nil {
					errCh <- err
				}
				defer f.Close()

				intervalChn := time.After(c.interval)

			OUTER:

				for {
					select {
					case log := <-logCh:
						// Append the line to the file
						if _, err = f.WriteString(log + "\n"); err != nil {
							errCh <- err
							break OUTER
						}
					// Stop collecting the logs after the interval specified
					case <-intervalChn:
						break OUTER
					}
				}

				wg.Done()
			}()
		}

		// Wait for all captures to complete
		wg.Wait()

		// Send down the timestamp for UI output
		successChan <- timestamp
	}

	go capture()

	for {
		select {
		case t := <-successChan:
			intervalCount++
			c.UI.Output(fmt.Sprintf("Capture successful %s (%d)", time.Unix(t, 0).Local().String(), intervalCount))
			go capture()
		case e := <-errCh:
			c.UI.Error(fmt.Sprintf("Capture failure %s", e))
		case <-durationChn:
			return nil
		case <-c.shutdownCh:
			return errors.New("stopping collection due to shutdown signal")
		}
	}
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

// createArchive walks the files in the temporary directory
// and creates a tar file that is gzipped with the contents
func (c *cmd) createArchive() error {
	f, err := os.Create(c.output + debugArchiveExtension)
	if err != nil {
		return fmt.Errorf("failed to create compressed archive: %s", err)
	}
	defer f.Close()

	g := gzip.NewWriter(f)
	defer g.Close()
	t := tar.NewWriter(f)
	defer t.Close()

	err = filepath.Walk(c.output, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk filepath for archive: %s", err)
		}

		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return fmt.Errorf("failed to create compressed archive header: %s", err)
		}

		header.Name = filepath.Join(filepath.Base(c.output), strings.TrimPrefix(file, c.output))

		if err := t.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write compressed archive header: %s", err)
		}

		// Only copy files
		if !fi.Mode().IsRegular() {
			return nil
		}

		f, err := os.Open(file)
		if err != nil {
			return fmt.Errorf("failed to open target files for archive: %s", err)
		}

		if _, err := io.Copy(t, f); err != nil {
			return fmt.Errorf("failed to copy files for archive: %s", err)
		}

		f.Close()

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk output path for archive: %s", err)
	}

	// Remove directory that has been archived
	err = os.RemoveAll(c.output)
	if err != nil {
		return fmt.Errorf("failed to remove archived directory: %s", err)
	}

	return nil
}

// defaultTargets specifies the list of all targets that
// will be captured by default
func (c *cmd) defaultTargets() []string {
	return append(c.dynamicTargets(), c.staticTargets()...)
}

// dynamicTargets returns all the supported targets
// that are retrieved at the interval specified
func (c *cmd) dynamicTargets() []string {
	return []string{"metrics", "logs", "pprof"}
}

// staticTargets returns all the supported targets
// that are retrieved at the start of the command execution
func (c *cmd) staticTargets() []string {
	return []string{"host", "agent", "cluster"}
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Records a debugging archive for operators"
const help = `
Usage: consul debug [options]

  Monitors a Consul agent for the specified period of time, recording
  information about the agent, cluster, and environment to an archive
  written to the specified path.

  If ACLs are enabled, an 'operator:read' token must be supplied in order
  to perform this operation.

  To create a debug archive in the current directory for the default
  duration and interval, capturing all information available:

      $ consul debug

  Flags can be used to customize the duration and interval of the
  operation. Note that the duration must be longer than the interval.

      $ consul debug -interval=20s -duration=1m

  The capture flag can be specified multiple times to limit information
  retrieved.

      $ consul debug -capture metrics -capture agent

  By default, the archive containing the debugging information is
  saved to the current directory as a .tar.gz file. The
  output path can be specified, as well as an option to disable
  archiving, leaving the directory intact.

      $ consul debug -output=/foo/bar/my-debugging -archive=false

  Note: Information collected by this command has the potential
  to be highly sensitive. We strongly recommend review of the
  data within the archive prior to transmitting it.

  For a full list of options and examples, please see the Consul
  documentation.
`
