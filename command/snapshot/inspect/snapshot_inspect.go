package inspect

import (
	"bytes"
	"compress/gzip"
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
	c.flags.BoolVar(&c.enhance, "enhance", false,
		"Adds more info")
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	var file string
	args = c.flags.Args()
	numArgs := len(args)

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
		if _, err := f.Seek(0, 0); err != nil {
			c.UI.Error(fmt.Sprintf("Error resetting file for enhancement, got %s", err))
		}
		decomp, err := gzip.NewReader(f)
		if err != nil {
			c.UI.Error(fmt.Sprintf("failed to decompress snapshot: %v", err))
			return 1
		}
		defer decomp.Close()

		stats, err := enhance(decomp)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error verifying snapshot: %s", err))
			return 1
		}
		b, err := c.readStats(stats)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error reading snapshot stats: %s", err))
			return 1
		}
		c.UI.Info(b.String())
		return 0
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

// enhance utilizes ReadSnapshot to create a summary
// of each messageType in a snapshot
func enhance(file io.Reader) (map[structs.MessageType]typeStats, error) {
	stats := make(map[structs.MessageType]typeStats)
	var offset int
	offset = 0
	cr := &countingReader{wrappedReader: file}
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

		size := cr.read - offset
		s.Sum += size
		s.Count++
		offset = cr.read
		stats[msg] = s
		return nil
	}
	if err := fsm.ReadSnapshot(cr, handler); err != nil {
		return nil, err
	}
	return stats, nil

}

func (c *cmd) readStats(stats map[structs.MessageType]typeStats) (bytes.Buffer, error) {
	// Output stats in size-order
	ss := make([]typeStats, 0, len(stats))

	for _, s := range stats {
		ss = append(ss, s)
	}

	// Sort the stat slice
	sort.Slice(ss, func(i, j int) bool { return ss[i].Sum > ss[j].Sum })

	var b bytes.Buffer
	tw := tabwriter.NewWriter(&b, 0, 3, 6, ' ', 0)
	for _, s := range ss {
		fmt.Fprintf(tw, "Type\t%s\n", s.Name)
		fmt.Fprintf(tw, "Count\t%d\n", s.Count)
		fmt.Fprintf(tw, "Size\t%s\n", ByteSize(uint64(s.Sum)))
	}
	if err := tw.Flush(); err != nil {
		c.UI.Error(fmt.Sprintf("Error rendering snapshot info: %s", err))
		return b, err
	}

	return b, nil

}

// ByteSize returns a human-readable byte string of the form 10M, 12.5K, and so forth.  The following units are available:
//	T: Terabyte
//	G: Gigabyte
//	M: Megabyte
//	K: Kilobyte
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
