package agent

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/consul/consul"
	"github.com/mitchellh/mapstructure"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Config is the configuration that can be set for an Agent.
// Some of this is configurable as CLI flags, but most must
// be set using a configuration file.
type Config struct {
	// Bootstrap is used to bring up the first Consul server, and
	// permits that node to elect itself leader
	Bootstrap bool

	// Datacenter is the datacenter this node is in. Defaults to dc1
	Datacenter string

	// DataDir is the directory to store our state in
	DataDir string

	// Encryption key to use for the Serf communication
	EncryptKey string

	// HTTP interface address
	HTTPAddr string

	// LogLevel is the level of the logs to putout
	LogLevel string

	// Node name is the name we use to advertise. Defaults to hostname.
	NodeName string

	// RPCAddr is the address and port to listen on for the
	// agent's RPC interface.
	RPCAddr string

	// BindAddr is the address that Consul's RPC and Serf's will
	// bind to. This address should be routable by all other hosts.
	SerfBindAddr string

	// SerfLanPort is the port we use for the lan-local serf cluster
	// This is used for all nodes.
	SerfLanPort int

	// SerfWanPort is the port we use for the wan serf cluster.
	// This is only for the Consul servers
	SerfWanPort int

	// ServerAddr is the address we use for Consul server communication.
	// Defaults to 0.0.0.0:8300
	ServerAddr string

	// Server controls if this agent acts like a Consul server,
	// or merely as a client. Servers have more state, take part
	// in leader election, etc.
	Server bool

	// LeaveOnTerm controls if Serf does a graceful leave when receiving
	// the TERM signal. Defaults false. This can be changed on reload.
	LeaveOnTerm bool `mapstructure:"leave_on_terminate"`

	// SkipLeaveOnInt controls if Serf skips a graceful leave when receiving
	// the INT signal. Defaults false. This can be changed on reload.
	SkipLeaveOnInt bool `mapstructure:"skip_leave_on_interrupt"`

	// ConsulConfig can either be provided or a default one created
	ConsulConfig *consul.Config
}

type dirEnts []os.FileInfo

// DefaultConfig is used to return a sane default configuration
func DefaultConfig() *Config {
	return &Config{
		Datacenter: consul.DefaultDC,
		HTTPAddr:   "127.0.0.1:8500",
		LogLevel:   "INFO",
		RPCAddr:    "127.0.0.1:8400",
		Server:     false,
	}
}

// EncryptBytes returns the encryption key configured.
func (c *Config) EncryptBytes() ([]byte, error) {
	return base64.StdEncoding.DecodeString(c.EncryptKey)
}

// DecodeConfig reads the configuration from the given reader in JSON
// format and decodes it into a proper Config structure.
func DecodeConfig(r io.Reader) (*Config, error) {
	var raw interface{}
	dec := json.NewDecoder(r)
	if err := dec.Decode(&raw); err != nil {
		return nil, err
	}

	// Decode
	var md mapstructure.Metadata
	var result Config
	msdec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Metadata: &md,
		Result:   &result,
	})
	if err != nil {
		return nil, err
	}

	if err := msdec.Decode(raw); err != nil {
		return nil, err
	}

	return &result, nil
}

// MergeConfig merges two configurations together to make a single new
// configuration.
func MergeConfig(a, b *Config) *Config {
	var result Config = *a

	// Copy the strings if they're set
	if b.Bootstrap {
		result.Bootstrap = true
	}
	if b.Datacenter != "" {
		result.Datacenter = b.Datacenter
	}
	if b.DataDir != "" {
		result.DataDir = b.DataDir
	}
	if b.EncryptKey != "" {
		result.EncryptKey = b.EncryptKey
	}
	if b.HTTPAddr != "" {
		result.HTTPAddr = b.HTTPAddr
	}
	if b.LogLevel != "" {
		result.LogLevel = b.LogLevel
	}
	if b.RPCAddr != "" {
		result.RPCAddr = b.RPCAddr
	}
	if b.SerfBindAddr != "" {
		result.SerfBindAddr = b.SerfBindAddr
	}
	if b.SerfLanPort > 0 {
		result.SerfLanPort = b.SerfLanPort
	}
	if b.SerfWanPort > 0 {
		result.SerfWanPort = b.SerfWanPort
	}
	if b.ServerAddr != "" {
		result.ServerAddr = b.ServerAddr
	}
	if b.Server == true {
		result.Server = b.Server
	}
	if b.LeaveOnTerm == true {
		result.LeaveOnTerm = true
	}
	if b.SkipLeaveOnInt == true {
		result.SkipLeaveOnInt = true
	}
	return &result
}

// ReadConfigPaths reads the paths in the given order to load configurations.
// The paths can be to files or directories. If the path is a directory,
// we read one directory deep and read any files ending in ".json" as
// configuration files.
func ReadConfigPaths(paths []string) (*Config, error) {
	result := new(Config)
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("Error reading '%s': %s", path, err)
		}

		fi, err := f.Stat()
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("Error reading '%s': %s", path, err)
		}

		if !fi.IsDir() {
			config, err := DecodeConfig(f)
			f.Close()

			if err != nil {
				return nil, fmt.Errorf("Error decoding '%s': %s", path, err)
			}

			result = MergeConfig(result, config)
			continue
		}

		contents, err := f.Readdir(-1)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("Error reading '%s': %s", path, err)
		}

		// Sort the contents, ensures lexical order
		sort.Sort(dirEnts(contents))

		for _, fi := range contents {
			// Don't recursively read contents
			if fi.IsDir() {
				continue
			}

			// If it isn't a JSON file, ignore it
			if !strings.HasSuffix(fi.Name(), ".json") {
				continue
			}

			subpath := filepath.Join(path, fi.Name())
			f, err := os.Open(subpath)
			if err != nil {
				return nil, fmt.Errorf("Error reading '%s': %s", subpath, err)
			}

			config, err := DecodeConfig(f)
			f.Close()

			if err != nil {
				return nil, fmt.Errorf("Error decoding '%s': %s", subpath, err)
			}

			result = MergeConfig(result, config)
		}
	}

	return result, nil
}

// Implement the sort interface for dirEnts
func (d dirEnts) Len() int {
	return len(d)
}

func (d dirEnts) Less(i, j int) bool {
	return d[i].Name() < d[j].Name()
}

func (d dirEnts) Swap(i, j int) {
	d[i], d[j] = d[j], d[i]
}
