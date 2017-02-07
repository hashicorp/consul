package command

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/hashicorp/consul/api"
	"github.com/mitchellh/cli"
	text "github.com/tonnerre/golang-text"
)

// maxLineLength is the maximum width of any line.
const maxLineLength int = 72

// FlagSetFlags is an enum to define what flags are present in the
// default FlagSet returned.
type FlagSetFlags uint

const (
	FlagSetNone FlagSetFlags = iota << 1
	FlagSetHTTP FlagSetFlags = iota << 1
)

type Meta struct {
	Ui    cli.Ui
	Flags FlagSetFlags

	flagSet *flag.FlagSet

	// These are the options which correspond to the HTTP API options
	httpAddr   string
	datacenter string
	token      string
	stale      bool
}

// HTTPClient returns a client with the parsed flags. It panics if the command
// does not accept HTTP flags or if the flags have not been parsed.
func (m *Meta) HTTPClient() (*api.Client, error) {
	if !m.hasHTTP() {
		panic("no http flags defined")
	}
	if !m.flagSet.Parsed() {
		panic("flags have not been parsed")
	}

	return api.NewClient(&api.Config{
		Datacenter: m.datacenter,
		Address:    m.httpAddr,
		Token:      m.token,
	})
}

// httpFlags is the list of flags that apply to HTTP connections.
func (m *Meta) httpFlags(f *flag.FlagSet) *flag.FlagSet {
	if f == nil {
		f = flag.NewFlagSet("", flag.ContinueOnError)
	}

	f.StringVar(&m.datacenter, "datacenter", "",
		"Name of the datacenter to query. If unspecified, this will default to "+
			"the datacenter of the queried agent.")
	f.StringVar(&m.httpAddr, "http-addr", "",
		"Address and port to the Consul HTTP agent. The value can be an IP "+
			"address or DNS address, but it must also include the port. This can "+
			"also be specified via the CONSUL_HTTP_ADDR environment variable. The "+
			"default value is 127.0.0.1:8500.")
	f.StringVar(&m.token, "token", "",
		"ACL token to use in the request. This can also be specified via the "+
			"CONSUL_HTTP_TOKEN environment variable. If unspecified, the query will "+
			"default to the token of the Consul agent at the HTTP address.")
	f.BoolVar(&m.stale, "stale", false,
		"Permit any Consul server (non-leader) to respond to this request. This "+
			"allows for lower latency and higher throughput, but can result in "+
			"stale data. This option has no effect on non-read operations. The "+
			"default value is false.")

	return f
}

// NewFlagSet creates a new flag set for the given command. It automatically
// generates help output and adds the appropriate API flags.
func (m *Meta) NewFlagSet(c cli.Command) *flag.FlagSet {
	f := flag.NewFlagSet("", flag.ContinueOnError)
	f.Usage = func() { m.Ui.Error(c.Help()) }

	if m.hasHTTP() {
		m.httpFlags(f)
	}

	errR, errW := io.Pipe()
	errScanner := bufio.NewScanner(errR)
	go func() {
		for errScanner.Scan() {
			m.Ui.Error(errScanner.Text())
		}
	}()
	f.SetOutput(errW)

	m.flagSet = f

	return f
}

// Parse is used to parse the underlying flag set.
func (m *Meta) Parse(args []string) error {
	return m.flagSet.Parse(args)
}

// Help returns the help for this flagSet.
func (m *Meta) Help() string {
	return m.helpFlagsFor(m.flagSet)
}

// hasHTTP returns true if this meta command contains HTTP flags.
func (m *Meta) hasHTTP() bool {
	return m.Flags&FlagSetHTTP != 0
}

// helpFlagsFor visits all flags in the given flag set and prints formatted
// help output. This function is sad because there's no "merging" of command
// line flags. We explicitly pull out our "common" options into another section
// by doing string comparisons :(.
func (m *Meta) helpFlagsFor(f *flag.FlagSet) string {
	httpFlags := m.httpFlags(nil)

	var out bytes.Buffer

	if f.NFlag() > 0 {
		printTitle(&out, "Command Options")
		f.VisitAll(func(f *flag.Flag) {
			// Skip HTTP flags as they will be grouped separately
			if flagContains(httpFlags, f) {
				return
			}
			printFlag(&out, f)
		})
	}

	if m.hasHTTP() {
		printTitle(&out, "HTTP API Options")
		httpFlags.VisitAll(func(f *flag.Flag) {
			printFlag(&out, f)
		})
	}

	return strings.TrimRight(out.String(), "\n")
}

// printTitle prints a consistently-formatted title to the given writer.
func printTitle(w io.Writer, s string) {
	fmt.Fprintf(w, "%s\n\n", s)
}

// printFlag prints a single flag to the given writer.
func printFlag(w io.Writer, f *flag.Flag) {
	example, _ := flag.UnquoteUsage(f)
	if example != "" {
		fmt.Fprintf(w, "  -%s=<%s>\n", f.Name, example)
	} else {
		fmt.Fprintf(w, "  -%s\n", f.Name)
	}

	indented := wrapAtLength(f.Usage, 5)
	fmt.Fprintf(w, "%s\n\n", indented)
}

// flagContains returns true if the given flag is contained in the given flag
// set or false otherwise.
func flagContains(fs *flag.FlagSet, f *flag.Flag) bool {
	var skip bool

	fs.VisitAll(func(hf *flag.Flag) {
		if skip {
			return
		}

		if f.Name == hf.Name && f.Usage == hf.Usage {
			skip = true
			return
		}
	})

	return skip
}

// wrapAtLength wraps the given text at the maxLineLength, taxing into account
// any provided left padding.
func wrapAtLength(s string, pad int) string {
	wrapped := text.Wrap(s, maxLineLength-pad)
	lines := strings.Split(wrapped, "\n")
	for i, line := range lines {
		lines[i] = strings.Repeat(" ", pad) + line
	}
	return strings.Join(lines, "\n")
}
