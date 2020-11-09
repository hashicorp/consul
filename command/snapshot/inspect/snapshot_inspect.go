package inspect

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

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
	UI     cli.Ui
	flags  *flag.FlagSet
	help   string
	format string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(
		&c.format,
		"format",
		PrettyFormat,
		fmt.Sprintf("Output format {%s}", strings.Join(GetSupportedFormats(), "|")))

	c.help = flags.Usage(help, c.flags)
}

// MetadataInfo is used for passing information
// through the formatter
type MetadataInfo struct {
	ID      string
	Size    int64
	Index   uint64
	Term    uint64
	Version raft.SnapshotVersion
}

// OutputFormat is used for passing information
// through the formatter
type OutputFormat struct {
	Meta      *MetadataInfo
	Stats     []typeStats
	TotalSize int
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

	formatter, err := NewFormatter(c.format)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error outputting enhanced snapshot data: %s", err))
		return 1
	}
	//Generate structs for the formatter with information we read in
	metaformat := &MetadataInfo{
		ID:      meta.ID,
		Size:    meta.Size,
		Index:   meta.Index,
		Term:    meta.Term,
		Version: meta.Version,
	}

	//Restructures stats given above to be human readable
	formattedStats := generatetypeStats(stats)

	in := &OutputFormat{
		Meta:      metaformat,
		Stats:     formattedStats,
		TotalSize: totalSize,
	}
	out, err := formatter.Format(in)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	c.UI.Output(out)
	return 0
}

type typeStats struct {
	Name  string
	Sum   int
	Count int
}

func generatetypeStats(info map[structs.MessageType]typeStats) []typeStats {
	ss := make([]typeStats, 0, len(info))

	for _, s := range info {
		ss = append(ss, s)
	}

	// Sort the stat slice
	sort.Slice(ss, func(i, j int) bool { return ss[i].Sum > ss[j].Sum })

	return ss
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
