package inspect

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/snapshot"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/raft"
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
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	var file string
	args = c.flags.Args()

	switch len(args) {
	case 0:
		c.UI.Error("Missing FILE argument")
		return 1
	case 1:
		file = args[0]
	default:
		c.UI.Error(fmt.Sprintf("Too many arguments (expected 1, got %d)", len(args)))
		return 1
	}

	// Open the file.
	f, err := os.Open(file)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error opening snapshot file: %s", err))
		return 1
	}
	defer f.Close()

	readFile, meta, err := snapshot.Read(hclog.New(nil), f)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error reading snapshot: %s", err))
		return 1
	}
	defer func() {
		if err := readFile.Close(); err != nil {
			c.UI.Error(fmt.Sprintf("Failed to close temp snapshot: %v", err))
		}
		if err := os.Remove(readFile.Name()); err != nil {
			c.UI.Error(fmt.Sprintf("Failed to clean up temp snapshot: %v", err))
		}
	}()

	stats, totalSize, err := enhance(readFile)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error extracting snapshot data: %s", err))
		return 1
	}
	// Outputs the original style of inspect information
	legacy, err := c.legacyStats(meta)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error outputting snapshot data: %s", err))
	}
	c.UI.Info(legacy.String())

	// Outputs the more detailed snapshot information
	enhanced, err := c.readStats(stats, totalSize)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error outputting enhanced snapshot data: %s", err))
		return 1
	}
	c.UI.Info(enhanced.String())

	return 0
}

// legacyStats outputs the expected stats from the original snapshot
// inspect command
func (c *cmd) legacyStats(meta *raft.SnapshotMeta) (bytes.Buffer, error) {
	var b bytes.Buffer
	tw := tabwriter.NewWriter(&b, 0, 2, 6, ' ', 0)
	fmt.Fprintf(tw, "ID\t%s\n", meta.ID)
	fmt.Fprintf(tw, "Size\t%d\n", meta.Size)
	fmt.Fprintf(tw, "Index\t%d\n", meta.Index)
	fmt.Fprintf(tw, "Term\t%d\n", meta.Term)
	fmt.Fprintf(tw, "Version\t%d\n", meta.Version)
	if err := tw.Flush(); err != nil {
		return b, err
	}
	return b, nil
}

type typeStats struct {
	Name  string
	Sum   int
	Count int
}

// countingReader helps keep track of the bytes we have read
// when reading snapshots
type countingReader struct {
	wrappedReader io.Reader
	read          int
}

func (r *countingReader) Read(p []byte) (n int, err error) {
	n, err = r.wrappedReader.Read(p)
	if err == nil {
		r.read += n
	}
	return n, err
}

// enhance utilizes ReadSnapshot to populate the struct with
// all of the snapshot's itemized data
func enhance(file io.Reader) (map[structs.MessageType]typeStats, int, error) {
	stats := make(map[structs.MessageType]typeStats)
	cr := &countingReader{wrappedReader: file}
	totalSize := 0
	handler := func(header *fsm.SnapshotHeader, msg structs.MessageType, dec *codec.Decoder) error {
		name := structs.MessageType.String(msg)
		s := stats[msg]
		if s.Name == "" {
			s.Name = name
		}
		var val interface{}
		err := dec.Decode(&val)
		if err != nil {
			return fmt.Errorf("failed to decode msg type %v, error %v", name, err)
		}

		size := cr.read - totalSize
		s.Sum += size
		s.Count++
		totalSize = cr.read
		stats[msg] = s
		return nil
	}
	if err := fsm.ReadSnapshot(cr, handler); err != nil {
		return nil, 0, err
	}
	return stats, totalSize, nil

}

// readStats takes the information generated from enhance and creates human
// readable output from it
func (c *cmd) readStats(stats map[structs.MessageType]typeStats, totalSize int) (bytes.Buffer, error) {
	// Output stats in size-order
	ss := make([]typeStats, 0, len(stats))

	for _, s := range stats {
		ss = append(ss, s)
	}

	// Sort the stat slice
	sort.Slice(ss, func(i, j int) bool { return ss[i].Sum > ss[j].Sum })

	var b bytes.Buffer

	tw := tabwriter.NewWriter(&b, 8, 8, 6, ' ', 0)
	fmt.Fprintln(tw, "\n Type\tCount\tSize\t")
	fmt.Fprintf(tw, " %s\t%s\t%s\t", "----", "----", "----")
	// For each different type generate new output
	for _, s := range ss {
		fmt.Fprintf(tw, "\n %s\t%d\t%s\t", s.Name, s.Count, ByteSize(uint64(s.Sum)))
	}
	fmt.Fprintf(tw, "\n %s\t%s\t%s\t", "----", "----", "----")
	fmt.Fprintf(tw, "\n Total\t\t%s\t", ByteSize(uint64(totalSize)))

	if err := tw.Flush(); err != nil {
		c.UI.Error(fmt.Sprintf("Error rendering snapshot info: %s", err))
		return b, err
	}

	return b, nil

}

// ByteSize returns a human-readable byte string of the form 10MB, 12.5KB, and so forth.  The following units are available:
//	TB: Terabyte
//	GB: Gigabyte
//	MB: Megabyte
//	KB: Kilobyte
//	B: Byte
// The unit that results in the smallest number greater than or equal to 1 is always chosen.
// From https://github.com/cloudfoundry/bytefmt/blob/master/bytes.go

const (
	BYTE = 1 << (10 * iota)
	KILOBYTE
	MEGABYTE
	GIGABYTE
	TERABYTE
)

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

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Displays information about a Consul snapshot file"
const help = `
Usage: consul snapshot inspect [options] FILE

  Displays information about a snapshot file on disk.

  To inspect the file "backup.snap":

    $ consul snapshot inspect backup.snap
  
  For a full list of options and examples, please see the Consul documentation.
`
