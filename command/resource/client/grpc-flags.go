// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"flag"
)

type GRPCFlags struct {
	address StringValue
}

// mergeFlagsIntoGRPCConfig merges flag values into grpc config
// caller has to parse the CLI args before loading them into flag values
func (f *GRPCFlags) mergeFlagsIntoGRPCConfig(c *GRPCConfig) {
	f.address.Merge(&c.Address)
}

func (f *GRPCFlags) ClientFlags() *flag.FlagSet {
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	fs.Var(&f.address, "grpc-addr",
		"The `address` and port of the Consul GRPC agent. The value can be an IP "+
			"address or DNS address, but it must also include the port. This can "+
			"also be specified via the CONSUL_GRPC_ADDR environment variable. The "+
			"default value is 127.0.0.1:8502. It supports TLS communication "+
			"by setting the environment variable CONSUL_GRPC_TLS=true.")
	return fs
}

type StringValue struct {
	v *string
}

// Set implements the flag.Value interface.
func (s *StringValue) Set(v string) error {
	if s.v == nil {
		s.v = new(string)
	}
	*(s.v) = v
	return nil
}

// String implements the flag.Value interface.
func (s *StringValue) String() string {
	var current string
	if s.v != nil {
		current = *(s.v)
	}
	return current
}

// Merge will overlay this value if it has been set.
func (s *StringValue) Merge(onto *string) {
	if s.v != nil {
		*onto = *(s.v)
	}
}
