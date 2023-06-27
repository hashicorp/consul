// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package debug

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mitchellh/cli"
	"golang.org/x/sync/errgroup"

	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
)

const (
	// debugInterval is the interval in which to capture dynamic information
	// when running debug
	debugInterval = 30 * time.Second

	// debugDuration is the total duration that debug runs before being
	// shut down
	debugDuration = 5 * time.Minute

	// debugDurationGrace is a period of time added to the specified
	// duration to allow intervals to capture within that time
	debugDurationGrace = 2 * time.Second

	// debugMinInterval is the minimum a user can configure the interval
	// to prevent accidental DOS
	debugMinInterval = 5 * time.Second

	// debugMinDuration is the minimum a user can configure the duration
	// to ensure that all information can be collected in time
	debugMinDuration = 10 * time.Second

	// debugArchiveExtension is the extension for archive files
	debugArchiveExtension = ".tar.gz"

	// debugProtocolVersion is the version of the package that is
	// generated. If this format changes interface, this version
	// can be incremented so clients can selectively support packages
	debugProtocolVersion = 1
)

func New(ui cli.Ui) *cmd {
	ui = &cli.PrefixedUi{
		OutputPrefix: "==> ",
		InfoPrefix:   "    ",
		ErrorPrefix:  "==> ",
		Ui:           ui,
	}

	c := &cmd{UI: ui}
	c.init()
	return c
}

type cmd struct {
	UI    cli.Ui
	flags *flag.FlagSet
	http  *flags.HTTPFlags
	help  string

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
	// timeNow is a shim for testing, it is used to generate the time used in
	// file paths.
	timeNow func() time.Time
}

// debugIndex is used to manage the summary of all data recorded
// during the debug, to be written to json at the end of the run
// and stored at the root. Each attribute corresponds to a file or files.
type debugIndex struct {
	// Version of the debug package
	Version int
	// Version of the target Consul agent
	AgentVersion string

	Interval string
	Duration string

	Targets []string
}

// timeDateformat is a modified version of time.RFC3339 which replaces colons with
// hyphens. This is to make it more convenient to untar these files, because
// tar assumes colons indicate the file is on a remote host, unless --force-local
// is used.
const timeDateFormat = "2006-01-02T15-04-05Z0700"

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)

	defaultFilename := fmt.Sprintf("consul-debug-%v", time.Now().Format(timeDateFormat))

	c.flags.Var((*flags.AppendSliceValue)(&c.capture), "capture",
		fmt.Sprintf("One or more types of information to capture. This can be used "+
			"to capture a subset of information, and defaults to capturing "+
			"everything available. Possible information for capture: %s. "+
			"This can be repeated multiple times.", strings.Join(defaultTargets, ", ")))
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
	c.help = flags.Usage(help, c.flags)

	c.validateTiming = true
	c.timeNow = func() time.Time {
		return time.Now().UTC()
	}
}

func (c *cmd) Run(args []string) int {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

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
	if c.captureTarget(targetMetrics) || c.captureTarget(targetLogs) || c.captureTarget(targetProfiles) {
		g := new(errgroup.Group)
		g.Go(func() error {
			return c.captureInterval(ctx)
		})
		g.Go(func() error {
			return c.captureLongRunning(ctx)
		})
		err = g.Wait()
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error encountered during collection: %v", err))
		}
	}

	// Record some information for the index at the root of the archive
	index := &debugIndex{
		Version:      debugProtocolVersion,
		AgentVersion: version,
		Interval:     c.interval.String(),
		Duration:     c.duration.String(),
		Targets:      c.capture,
	}
	if err := writeJSONFile(filepath.Join(c.output, "index.json"), index); err != nil {
		c.UI.Error(fmt.Sprintf("Error creating index document: %v", err))
		return 1
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

	// If none are specified we will collect information from
	// all by default
	if len(c.capture) == 0 {
		c.capture = make([]string, len(defaultTargets))
		copy(c.capture, defaultTargets)
	}

	// If EnableDebug is not true, skip collecting pprof
	enableDebug, ok := self["DebugConfig"]["EnableDebug"].(bool)
	if !ok {
		return "", fmt.Errorf("agent response did not contain EnableDebug key")
	}
	if !enableDebug {
		cs := c.capture
		for i := 0; i < len(cs); i++ {
			if cs[i] == "pprof" {
				c.capture = append(cs[:i], cs[i+1:]...)
				i--
				c.UI.Warn("[WARN] Unable to capture pprof. Set enable_debug to true on target agent to enable profiling.")
			}
		}
	}

	for _, t := range c.capture {
		if !allowedTarget(t) {
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
	var errs error

	if c.captureTarget(targetHost) {
		host, err := c.client.Agent().Host()
		if err != nil {
			errs = multierror.Append(errs, err)
		}
		if err := writeJSONFile(filepath.Join(c.output, targetHost+".json"), host); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	if c.captureTarget(targetAgent) {
		agent, err := c.client.Agent().Self()
		if err != nil {
			errs = multierror.Append(errs, err)
		}
		if err := writeJSONFile(filepath.Join(c.output, targetAgent+".json"), agent); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	if c.captureTarget(targetMembers) {
		members, err := c.client.Agent().Members(true)
		if err != nil {
			errs = multierror.Append(errs, err)
		}
		if err := writeJSONFile(filepath.Join(c.output, targetMembers+".json"), members); err != nil {
			errs = multierror.Append(errs, err)
		}
	}
	return errs
}

func writeJSONFile(filename string, content interface{}) error {
	marshaled, err := json.MarshalIndent(content, "", "\t")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, marshaled, 0644)
}

// captureInterval blocks for the duration of the command
// specified by the duration flag, capturing the dynamic
// targets at the interval specified
func (c *cmd) captureInterval(ctx context.Context) error {
	intervalChn := time.NewTicker(c.interval)
	defer intervalChn.Stop()
	durationChn := time.After(c.duration)
	intervalCount := 0

	c.UI.Output(fmt.Sprintf("Beginning capture interval %s (%d)", time.Now().UTC(), intervalCount))

	err := captureShortLived(c)
	if err != nil {
		return err
	}
	c.UI.Output(fmt.Sprintf("Capture successful %s (%d)", time.Now().UTC(), intervalCount))
	for {
		select {
		case t := <-intervalChn.C:
			intervalCount++
			err := captureShortLived(c)
			if err != nil {
				return err
			}
			c.UI.Output(fmt.Sprintf("Capture successful %s (%d)", t.UTC(), intervalCount))
		case <-durationChn:
			intervalChn.Stop()
			return nil
		case <-ctx.Done():
			return errors.New("stopping collection due to shutdown signal")
		}
	}
}

func captureShortLived(c *cmd) error {
	g := new(errgroup.Group)

	if c.captureTarget(targetProfiles) {
		dir, err := makeIntervalDir(c.output, c.timeNow())
		if err != nil {
			return err
		}

		g.Go(func() error {
			return c.captureHeap(dir)
		})

		g.Go(func() error {
			return c.captureGoRoutines(dir)
		})
	}
	return g.Wait()
}

func makeIntervalDir(base string, now time.Time) (string, error) {
	dir := filepath.Join(base, now.Format(timeDateFormat))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory %v: %w", dir, err)
	}
	return dir, nil
}

func (c *cmd) captureLongRunning(ctx context.Context) error {
	g := new(errgroup.Group)

	if c.captureTarget(targetProfiles) {
		g.Go(func() error {
			// use ctx without a timeout to allow the profile to finish sending
			return c.captureProfile(ctx, c.duration.Seconds())
		})

		g.Go(func() error {
			// use ctx without a timeout to allow the trace to finish sending
			return c.captureTrace(ctx, int(c.interval.Seconds()))
		})
	}
	if c.captureTarget(targetLogs) {
		g.Go(func() error {
			ctx, cancel := context.WithTimeout(ctx, c.duration)
			defer cancel()
			return c.captureLogs(ctx)
		})
	}
	if c.captureTarget(targetMetrics) {
		g.Go(func() error {
			ctx, cancel := context.WithTimeout(ctx, c.duration)
			defer cancel()
			return c.captureMetrics(ctx)
		})
	}

	return g.Wait()
}

func (c *cmd) captureGoRoutines(outputDir string) error {
	gr, err := c.client.Debug().Goroutine()
	if err != nil {
		return fmt.Errorf("failed to collect goroutine profile: %w", err)
	}

	return os.WriteFile(filepath.Join(outputDir, "goroutine.prof"), gr, 0644)
}

func (c *cmd) captureTrace(ctx context.Context, duration int) error {
	prof, err := c.client.Debug().PProf(ctx, "trace", duration)
	if err != nil {
		return fmt.Errorf("failed to collect cpu profile: %w", err)
	}
	defer prof.Close()

	r := bufio.NewReader(prof)
	fh, err := os.Create(filepath.Join(c.output, "trace.out"))
	if err != nil {
		return err
	}
	defer fh.Close()
	_, err = r.WriteTo(fh)
	return err
}

func (c *cmd) captureProfile(ctx context.Context, s float64) error {
	prof, err := c.client.Debug().PProf(ctx, "profile", int(s))
	if err != nil {
		return fmt.Errorf("failed to collect cpu profile: %w", err)
	}
	defer prof.Close()

	r := bufio.NewReader(prof)
	fh, err := os.Create(filepath.Join(c.output, "profile.prof"))
	if err != nil {
		return err
	}
	defer fh.Close()
	_, err = r.WriteTo(fh)
	return err
}

func (c *cmd) captureHeap(outputDir string) error {
	heap, err := c.client.Debug().Heap()
	if err != nil {
		return fmt.Errorf("failed to collect heap profile: %w", err)
	}

	return os.WriteFile(filepath.Join(outputDir, "heap.prof"), heap, 0644)
}

func (c *cmd) captureLogs(ctx context.Context) error {
	logCh, err := c.client.Agent().Monitor("TRACE", ctx.Done(), nil)
	if err != nil {
		return err
	}

	// Create the log file for writing
	f, err := os.Create(filepath.Join(c.output, "consul.log"))
	if err != nil {
		return err
	}
	defer f.Close()

	for {
		select {
		case log := <-logCh:
			if log == "" {
				return nil
			}
			if _, err = f.WriteString(log + "\n"); err != nil {
				return err
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func (c *cmd) captureMetrics(ctx context.Context) error {
	stream, err := c.client.Agent().MetricsStream(ctx)
	if err != nil {
		return err
	}
	defer stream.Close()

	fh, err := os.Create(filepath.Join(c.output, "metrics.json"))
	if err != nil {
		return fmt.Errorf("failed to create metrics file: %w", err)
	}
	defer fh.Close()

	b := bufio.NewReader(stream)
	_, err = b.WriteTo(fh)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("failed to copy metrics to file: %w", err)
	}
	return nil
}

// allowedTarget returns true if the target is a recognized name of a capture
// target.
func allowedTarget(target string) bool {
	for _, dt := range defaultTargets {
		if dt == target {
			return true
		}
	}
	for _, t := range deprecatedTargets {
		if t == target {
			return true
		}
	}
	return false
}

// captureTarget returns true if the target capture type is enabled.
func (c *cmd) captureTarget(target string) bool {
	for _, dt := range c.capture {
		if dt == target {
			return true
		}
		if target == targetMembers && dt == targetCluster {
			return true
		}
	}
	return false
}

// createArchive walks the files in the temporary directory
// and creates a tar file that is gzipped with the contents
func (c *cmd) createArchive() error {
	path := c.output + debugArchiveExtension

	tempName, err := c.createArchiveTemp(path)
	if err != nil {
		return err
	}

	if err := os.Rename(tempName, path); err != nil {
		return err
	}
	// fsync the dir to make the rename stick
	if err := syncParentDir(path); err != nil {
		return err
	}

	// Remove directory that has been archived
	if err := os.RemoveAll(c.output); err != nil {
		return fmt.Errorf("failed to remove archived directory: %s", err)
	}

	return nil
}

func syncParentDir(name string) error {
	f, err := os.Open(filepath.Dir(name))
	if err != nil {
		return err
	}
	defer f.Close()

	return f.Sync()
}

func (c *cmd) createArchiveTemp(path string) (tempName string, err error) {
	dir := filepath.Dir(path)
	name := filepath.Base(path)

	f, err := os.CreateTemp(dir, name+".tmp")
	if err != nil {
		return "", fmt.Errorf("failed to create compressed temp archive: %s", err)
	}

	g := gzip.NewWriter(f)
	t := tar.NewWriter(g)

	tempName = f.Name()

	cleanup := func(err error) (string, error) {
		_ = t.Close()
		_ = g.Close()
		_ = f.Close()
		_ = os.Remove(tempName)
		return "", err
	}

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

		return f.Close()
	})

	if err != nil {
		return cleanup(fmt.Errorf("failed to walk output path for archive: %s", err))
	}

	// Explicitly close things in the correct order (tar then gzip) so we
	// know if they worked.
	if err := t.Close(); err != nil {
		return cleanup(err)
	}
	if err := g.Close(); err != nil {
		return cleanup(err)
	}

	// Guarantee that the contents of the temp file are flushed to disk.
	if err := f.Sync(); err != nil {
		return cleanup(err)
	}

	// Close the temp file and go back to the wrapper function for the rest.
	if err := f.Close(); err != nil {
		return cleanup(err)
	}

	return tempName, nil
}

const (
	targetMetrics  = "metrics"
	targetLogs     = "logs"
	targetProfiles = "pprof"
	targetHost     = "host"
	targetAgent    = "agent"
	targetMembers  = "members"
	// targetCluster is the now deprecated name for targetMembers
	targetCluster = "cluster"
)

// defaultTargets specifies the list of targets that will be captured by default
var defaultTargets = []string{
	targetMetrics,
	targetLogs,
	targetProfiles,
	targetHost,
	targetAgent,
	targetMembers,
}

var deprecatedTargets = []string{targetCluster}

func (c *cmd) Synopsis() string {
	return "Records a debugging archive for operators"
}

func (c *cmd) Help() string {
	return c.help
}

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

  The command stores captured data at the configured output path
  through the duration, and will archive the data at the same
  path if interrupted.

  Flags can be used to customize the duration and interval of the
  operation. Duration is the total time to capture data for from the target
  agent and interval controls how often dynamic data such as metrics
  are scraped.

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
  to be highly sensitive. Sensitive material such as ACL tokens and
  other commonly secret material are redacted automatically, but we
  strongly recommend review of the data within the archive prior to
  transmitting it.

  For a full list of options and examples, please see the Consul
  documentation.
`
