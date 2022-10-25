package agent

import (
	"encoding/json"

	"github.com/pkg/errors"

	"github.com/hashicorp/consul/agent/config"
)

type Builder struct {
	config.Config
}

// NewConfigBuilder instantiates a builder object with sensible defaults for a single consul instance
// This includes the following:
// * default ports with no plaintext options
// * debug logging
// * single server with bootstrap
// * bind to all interfaces, advertise on 'eth0'
// * connect enabled
func NewConfigBuilder() *Builder {
	b := &Builder{
		config.Config{
			AdvertiseAddrLAN: stringToPointer(`{{ GetInterfaceIP "eth0" }}`),
			BindAddr:         stringToPointer("0.0.0.0"),
			Bootstrap:        boolToPointer(true),
			ClientAddr:       stringToPointer("0.0.0.0"),
			Connect: config.Connect{
				Enabled: boolToPointer(true),
			},
			LogLevel:   stringToPointer("DEBUG"),
			ServerMode: boolToPointer(true),
		},
	}

	// These are the default ports, disabling plaintext transport
	b.Config.Ports = config.Ports{
		DNS:     intToPointer(8600),
		HTTP:    nil,
		HTTPS:   intToPointer(8501),
		GRPC:    intToPointer(8502),
		GRPCTLS: intToPointer(8503),
		SerfLAN: intToPointer(8301),
		SerfWAN: intToPointer(8302),
		Server:  intToPointer(8300),
	}

	return b
}

func (b *Builder) Bootstrap(servers int) *Builder {
	if servers < 1 {
		b.Config.Bootstrap = nil
		b.Config.BootstrapExpect = nil
	} else if servers == 1 {
		b.Config.Bootstrap = boolToPointer(true)
		b.Config.BootstrapExpect = nil
	} else {
		b.Config.Bootstrap = nil
		b.Config.BootstrapExpect = intToPointer(servers)
	}
	return b
}

func (b *Builder) Client() *Builder {
	b.Config.Ports.Server = nil
	b.Config.ServerMode = nil
	b.Config.Bootstrap = nil
	b.Config.BootstrapExpect = nil
	return b
}

func (b *Builder) Datacenter(name string) *Builder {
	b.Config.Datacenter = stringToPointer(name)
	return b
}

func (b *Builder) Peering(enable bool) *Builder {
	b.Config.Peering = config.Peering{
		Enabled: boolToPointer(enable),
	}
	return b
}

func (b *Builder) Telemetry(statSite string) *Builder {
	b.Config.Telemetry = config.Telemetry{
		StatsiteAddr: stringToPointer(statSite),
	}
	return b
}

// ToString renders the builders configuration into a string
// representation of the json config file for agents.
// DANGER! Some fields may not have json tags in the Agent Config.
// You may need to add these yourself.
func (b *Builder) ToString() (string, error) {
	out, err := json.MarshalIndent(b.Config, "", "  ")
	if err != nil {
		return "", errors.Wrap(err, "could not marshall builder")
	}
	return string(out), nil
}

func intToPointer(i int) *int {
	return &i
}

func boolToPointer(b bool) *bool {
	return &b
}

func stringToPointer(s string) *string {
	return &s
}
