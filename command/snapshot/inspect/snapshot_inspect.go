package inspect

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/hashicorp/consul-net-rpc/go-msgpack/codec"
	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/snapshot"
	"github.com/hashicorp/go-hclog"
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

	// flags
	kvDetails bool
	kvDepth   int
	kvFilter  string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.BoolVar(&c.kvDetails, "kvdetails", false,
		"Provides a detailed KV space usage breakdown for any KV data that's been stored.")
	c.flags.IntVar(&c.kvDepth, "kvdepth", 2,
		"Can only be used with -kvdetails. The key prefix depth used to breakdown KV store data. Defaults to 2.")
	c.flags.StringVar(&c.kvFilter, "kvfilter", "",
		"Can only be used with -kvdetails. Limits KV key breakdown using this prefix filter.")
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

// SnapshotInfo is used for passing snapshot stat
// information between functions
type SnapshotInfo struct {
	Meta        MetadataInfo
	Stats       map[structs.MessageType]typeStats
	StatsKV     map[string]typeStats
	TotalSize   int
	TotalSizeKV int
}

// OutputFormat is used for passing information
// through the formatter
type OutputFormat struct {
	Meta        *MetadataInfo
	Stats       []typeStats
	StatsKV     []typeStats
	TotalSize   int
	TotalSizeKV int
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

	var readFile *os.File
	var meta *raft.SnapshotMeta

	if strings.ToLower(path.Base(file)) == "state.bin" {
		// This is an internal raw raft snapshot not a gzipped archive one
		// downloaded from the API, we can read it directly
		readFile = f

		// Assume the meta is colocated and error if not.
		metaRaw, err := os.ReadFile(path.Join(path.Dir(file), "meta.json"))
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error reading meta.json from internal snapshot dir: %s", err))
			return 1
		}
		var metaDecoded raft.SnapshotMeta
		err = json.Unmarshal(metaRaw, &metaDecoded)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error parsing meta.json from internal snapshot dir: %s", err))
			return 1
		}
		meta = &metaDecoded
	} else {
		readFile, meta, err = snapshot.Read(hclog.New(nil), f)
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
	}

	info, err := c.enhance(readFile)
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
	formattedStats := generateStats(info)
	formattedStatsKV := generateKVStats(info)

	in := &OutputFormat{
		Meta:        metaformat,
		Stats:       formattedStats,
		StatsKV:     formattedStatsKV,
		TotalSize:   info.TotalSize,
		TotalSizeKV: info.TotalSizeKV,
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

// generateStats formats the stats for the output struct
// that's used to produce the printed output the user sees.
func generateStats(info SnapshotInfo) []typeStats {
	ss := make([]typeStats, 0, len(info.Stats))

	for _, s := range info.Stats {
		ss = append(ss, s)
	}

	ss = sortTypeStats(ss)

	return ss
}

// generateKVStats reformats the KV stats to work with
// the output struct that's used to produce the printed
// output the user sees.
func generateKVStats(info SnapshotInfo) []typeStats {
	kvLen := len(info.StatsKV)
	if kvLen > 0 {
		ks := make([]typeStats, 0, kvLen)

		for _, s := range info.StatsKV {
			ks = append(ks, s)
		}

		ks = sortTypeStats(ks)

		return ks
	}

	return nil
}

// sortTypeStats sorts the stat slice by size and then
// alphabetically in the case the size is identical
func sortTypeStats(stats []typeStats) []typeStats {
	sort.Slice(stats, func(i, j int) bool {
		// sort alphabetically if size is equal
		if stats[i].Sum == stats[j].Sum {
			return stats[i].Name < stats[j].Name
		}

		return stats[i].Sum > stats[j].Sum
	})

	return stats
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
func (c *cmd) enhance(file io.Reader) (SnapshotInfo, error) {
	info := SnapshotInfo{
		Stats:       make(map[structs.MessageType]typeStats),
		StatsKV:     make(map[string]typeStats),
		TotalSize:   0,
		TotalSizeKV: 0,
	}
	cr := &countingReader{wrappedReader: file}
	handler := func(header *fsm.SnapshotHeader, msg structs.MessageType, dec *codec.Decoder) error {
		name := structs.MessageType.String(msg)
		s := info.Stats[msg]
		if s.Name == "" {
			s.Name = name
		}

		var val interface{}
		err := dec.Decode(&val)
		if err != nil {
			return fmt.Errorf("failed to decode msg type %v, error %v", name, err)
		}

		size := cr.read - info.TotalSize
		s.Sum += size
		s.Count++
		info.TotalSize = cr.read
		info.Stats[msg] = s

		c.kvEnhance(s.Name, val, size, &info)

		return nil
	}
	if err := fsm.ReadSnapshot(cr, handler); err != nil {
		return info, err
	}
	return info, nil

}

// kvEnhance populates the struct with all of the snapshot's
// size information for KV data stored in it
func (c *cmd) kvEnhance(keyType string, val interface{}, size int, info *SnapshotInfo) {
	if c.kvDetails {
		if keyType != "KVS" {
			return
		}

		// have to coerce this into a usable type here or this won't work
		keyVal := val.(map[string]interface{})
		for k, v := range keyVal {
			// we only care about the entry on the key specifically
			// related to the key name, so skip all others
			if k != "Key" {
				continue
			}

			// check for whether a filter is specified. if it is, skip
			// any keys that don't match.
			if len(c.kvFilter) > 0 && !strings.HasPrefix(v.(string), c.kvFilter) {
				break
			}

			split := strings.Split(v.(string), "/")

			// handle the situation where the key is shorter than
			// the specified depth.
			actualDepth := c.kvDepth
			if c.kvDepth > len(split) {
				actualDepth = len(split)
			}
			prefix := strings.Join(split[0:actualDepth], "/")
			kvs := info.StatsKV[prefix]
			if kvs.Name == "" {
				kvs.Name = prefix
			}

			kvs.Sum += size
			kvs.Count++
			info.TotalSizeKV += size
			info.StatsKV[prefix] = kvs
		}
	}
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
