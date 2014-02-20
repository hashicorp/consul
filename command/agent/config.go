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
	"time"
)

// Config is the configuration that can be set for an Agent.
// Some of this is configurable as CLI flags, but most must
// be set using a configuration file.
type Config struct {
	// AEInterval controls the anti-entropy interval. This is how often
	// the agent attempts to reconcile it's local state with the server'
	// representation of our state. Defaults to every 60s.
	AEInterval time.Duration `mapstructure:"-"`

	// Bootstrap is used to bring up the first Consul server, and
	// permits that node to elect itself leader
	Bootstrap bool `mapstructure:"bootstrap"`

	// Datacenter is the datacenter this node is in. Defaults to dc1
	Datacenter string `mapstructure:"datacenter"`

	// DataDir is the directory to store our state in
	DataDir string `mapstructure:"data_dir"`

	// DNSAddr is the address of the DNS server for the agent
	DNSAddr string `mapstructure:"dns_addr"`

	// DNSRecursor can be set to allow the DNS server to recursively
	// resolve non-consul domains
	DNSRecursor string `mapstructure:"recursor"`

	// Domain is the DNS domain for the records. Defaults to "consul."
	Domain string `mapstructure:"domain"`

	// Encryption key to use for the Serf communication
	EncryptKey string `mapstructure:"encrypt"`

	// HTTP interface address
	HTTPAddr string `mapstructure:"http_addr"`

	// LogLevel is the level of the logs to putout
	LogLevel string `mapstructure:"log_level"`

	// Node name is the name we use to advertise. Defaults to hostname.
	NodeName string `mapstructure:"node_name"`

	// RPCAddr is the address and port to listen on for the
	// agent's RPC interface.
	RPCAddr string `mapstructure:"rpc_addr"`

	// BindAddr is the address that Consul's RPC and Serf's will
	// bind to. This address should be routable by all other hosts.
	SerfBindAddr string `mapstructure:"serf_bind_addr"`

	// SerfLanPort is the port we use for the lan-local serf cluster
	// This is used for all nodes.
	SerfLanPort int `mapstructure:"serf_lan_port"`

	// SerfWanPort is the port we use for the wan serf cluster.
	// This is only for the Consul servers
	SerfWanPort int `mapstructure:"serf_wan_port"`

	// ServerAddr is the address we use for Consul server communication.
	// Defaults to 0.0.0.0:8300
	ServerAddr string `mapstructure:"server_addr"`

	// AdvertiseAddr is the address we use for advertising our Serf,
	// and Consul RPC IP. If not specified, the first private IP we
	// find is used.
	AdvertiseAddr string `mapstructure:"advertise_addr"`

	// Server controls if this agent acts like a Consul server,
	// or merely as a client. Servers have more state, take part
	// in leader election, etc.
	Server bool `mapstructure:"server"`

	// LeaveOnTerm controls if Serf does a graceful leave when receiving
	// the TERM signal. Defaults false. This can be changed on reload.
	LeaveOnTerm bool `mapstructure:"leave_on_terminate"`

	// SkipLeaveOnInt controls if Serf skips a graceful leave when receiving
	// the INT signal. Defaults false. This can be changed on reload.
	SkipLeaveOnInt bool `mapstructure:"skip_leave_on_interrupt"`

	// StatsiteAddr is the address of a statsite instance. If provided,
	// metrics will be streamed to that instance.
	StatsiteAddr string `mapstructure:"statsite_addr"`

	// Checks holds the provided check definitions
	Checks []*CheckDefinition `mapstructure:"-"`

	// Services holds the provided service definitions
	Services []*ServiceDefinition `mapstructure:"-"`

	// ConsulConfig can either be provided or a default one created
	ConsulConfig *consul.Config `mapstructure:"-"`
}

type dirEnts []os.FileInfo

// DefaultConfig is used to return a sane default configuration
func DefaultConfig() *Config {
	return &Config{
		AEInterval:  time.Minute,
		Datacenter:  consul.DefaultDC,
		DNSAddr:     "127.0.0.1:8600",
		Domain:      "consul.",
		HTTPAddr:    "127.0.0.1:8500",
		LogLevel:    "INFO",
		RPCAddr:     "127.0.0.1:8400",
		SerfLanPort: consul.DefaultLANSerfPort,
		SerfWanPort: consul.DefaultWANSerfPort,
		Server:      false,
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
	var result Config
	dec := json.NewDecoder(r)
	if err := dec.Decode(&raw); err != nil {
		return nil, err
	}

	// Check the result type
	if obj, ok := raw.(map[string]interface{}); ok {
		// Check for a "service" or "check" key, meaning
		// this is actually a definition entry
		if sub, ok := obj["service"]; ok {
			service, err := DecodeServiceDefinition(sub)
			result.Services = append(result.Services, service)
			return &result, err
		} else if sub, ok := obj["check"]; ok {
			check, err := DecodeCheckDefinition(sub)
			result.Checks = append(result.Checks, check)
			return &result, err
		}
	}

	// Decode
	var md mapstructure.Metadata
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

// DecodeServiceDefinition is used to decode a service definition
func DecodeServiceDefinition(raw interface{}) (*ServiceDefinition, error) {
	var sub interface{}
	rawMap, ok := raw.(map[string]interface{})
	if !ok {
		goto AFTER_FIX
	}
	sub, ok = rawMap["check"]
	if !ok {
		goto AFTER_FIX
	}
	if err := FixupCheckType(sub); err != nil {
		return nil, err
	}
AFTER_FIX:
	var md mapstructure.Metadata
	var result ServiceDefinition
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

func FixupCheckType(raw interface{}) error {
	// Handle decoding of time durations
	rawMap, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}
	if ttl, ok := rawMap["ttl"]; ok {
		ttlS, ok := ttl.(string)
		if ok {
			if dur, err := time.ParseDuration(ttlS); err != nil {
				return err
			} else {
				rawMap["ttl"] = dur
			}
		}
	}
	if interval, ok := rawMap["interval"]; ok {
		intervalS, ok := interval.(string)
		if ok {
			if dur, err := time.ParseDuration(intervalS); err != nil {
				return err
			} else {
				rawMap["interval"] = dur
			}
		}
	}
	return nil
}

// DecodeCheckDefinition is used to decode a check definition
func DecodeCheckDefinition(raw interface{}) (*CheckDefinition, error) {
	if err := FixupCheckType(raw); err != nil {
		return nil, err
	}
	var md mapstructure.Metadata
	var result CheckDefinition
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
	if b.DNSAddr != "" {
		result.DNSAddr = b.DNSAddr
	}
	if b.DNSRecursor != "" {
		result.DNSRecursor = b.DNSRecursor
	}
	if b.Domain != "" {
		result.Domain = b.Domain
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
	if b.NodeName != "" {
		result.NodeName = b.NodeName
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
	if b.AdvertiseAddr != "" {
		result.AdvertiseAddr = b.AdvertiseAddr
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
	if b.Checks != nil {
		result.Checks = append(result.Checks, b.Checks...)
	}
	if b.Services != nil {
		result.Services = append(result.Services, b.Services...)
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
