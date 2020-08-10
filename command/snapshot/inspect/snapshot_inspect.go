package inspect

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/fsm"
	"github.com/hashicorp/consul/snapshot"
	"github.com/hashicorp/go-msgpack/codec"
	"github.com/mitchellh/cli"
)

func New(ui cli.Ui) *cmd {
	c := &cmd{UI: ui}
	c.init()
	return c
}

type cmd struct {
	UI    cli.Ui
	flags *flag.FlagSet
	help  string

	//flags
	enhance bool
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	//TODO(schristoff): better flag info would be good
	c.flags.BoolVar(&c.enhance, "verbose", false,
		"Adds more info")
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	var file string
	//TODO (schristoff): How do other flags handle this? Find other flags that take in
	//a file without a file flag and then have other flags.
	args = c.flags.Args()
	numArgs := len(args)

	//Check to make sure we have at least one argument
	if numArgs != 1 || numArgs != 2 {
		c.UI.Error(fmt.Sprint("Expected at least one argument, got %d", numArgs))
		return 1
	}

	//Set the last* argument to the file name
	file = args[numArgs-1]
	// Open the file.
	f, err := os.Open(file)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error opening snapshot file: %s", err))
		return 1
	}
	defer f.Close()

	meta, err := snapshot.Verify(f)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error verifying snapshot: %s", err))
		return 1
	}

	if c.enhance {
		err := Enhance(f)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error verifying snapshot: %s", err))
			return 1
		}

	}
	var b bytes.Buffer
	tw := tabwriter.NewWriter(&b, 0, 2, 6, ' ', 0)
	fmt.Fprintf(tw, "ID\t%s\n", meta.ID)
	fmt.Fprintf(tw, "Size\t%d\n", meta.Size)
	fmt.Fprintf(tw, "Index\t%d\n", meta.Index)
	fmt.Fprintf(tw, "Term\t%d\n", meta.Term)
	fmt.Fprintf(tw, "Version\t%d\n", meta.Version)
	if err = tw.Flush(); err != nil {
		c.UI.Error(fmt.Sprintf("Error rendering snapshot info: %s", err))
		return 1
	}

	c.UI.Info(b.String())

	return 0
}

type typeStats struct {
	Name       string
	Sum, Count int
}

// This should return the summary of KBs
func Enhance(file io.Reader) error {
	//We need to pass in an io.ReaderCloser and a handler to the
	// ReadSnapshot func

	//The handler reads each message and then has to do x thing with each
	// message

	//First step: make a map of typeStats, get typeStat will hold the stuff for
	//each  message we get, stats will hold *all* of it
	stats := make(map[structs.MessageType]typeStats)
	handler := func(header *snapshotHeader, msg structs.MessageType, dec *codec.Decoder) error {
		//Lookup msg type and see what the string name is
		name := structs.MessageType.String(msg)

		//Do we already know about this type of msg?
		if _, ok := stats[msg]; ok {
			//increment sum
			stats[msg(name[1])]++
		}
		//Now we know it's not real so let's make it
		s := stats[msg(string[1])]

	}
	fsm.ReadSnapshot(file, handler)
	return err
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const (
	BYTE = 1 << (10 * iota)
	KILOBYTE
	MEGABYTE
	GIGABYTE
	TERABYTE
)

// ByteSize returns a human-readable byte string of the form 10M, 12.5K, and so forth.  The following units are available:
//	T: Terabyte
//	G: Gigabyte
//	M: Megabyte
//	K: Kilobyte
//	B: Byte
// The unit that results in the smallest number greater than or equal to 1 is always chosen.
// From https://github.com/cloudfoundry/bytefmt/blob/master/bytes.go
func ByteSize(bytes uint64) string {
	unit := ""
	value := float64(bytes)

	switch {
	case bytes >= TERABYTE:
		unit = "TB"
		value = value / TERABYTE
	case bytes >= GIGABYTE:
		unit = "GB"
		value = value / GIGABYTE
	case bytes >= MEGABYTE:
		unit = "MB"
		value = value / MEGABYTE
	case bytes >= KILOBYTE:
		unit = "KB"
		value = value / KILOBYTE
	case bytes >= BYTE:
		unit = "B"
	case bytes == 0:
		return "0"
	}

	result := strconv.FormatFloat(value, 'f', 1, 64)
	result = strings.TrimSuffix(result, ".0")
	return result + unit
}

const synopsis = "Displays information about a Consul snapshot file"
const help = `
Usage: consul snapshot inspect [options] FILE

  Displays information about a snapshot file on disk.

  To inspect the file "backup.snap":

    $ consul snapshot inspect backup.snap

  For a full list of options and examples, please see the Consul documentation.
`
