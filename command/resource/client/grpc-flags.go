// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"flag"
	"strings"
)

type GRPCFlags struct {
	address StringValue
	grpcTLS BoolValue
	caFile  StringValue
	caPath  StringValue
}

// MergeFlagsIntoGRPCConfig merges flag values into grpc config
// caller has to parse the CLI args before loading them into flag values
// flag values take precedence over the environment values
func (f *GRPCFlags) MergeFlagsIntoGRPCConfig(c *GRPCConfig) {
	if strings.HasPrefix(strings.ToLower(f.address.String()), "https://") {
		c.GRPCTLS = true
	}
	f.address.Set(removeSchemaFromGRPCAddress(f.address.String()))
	f.address.Merge(&c.Address)
	// won't overwrite the value if it's false
	if *f.grpcTLS.v {
		f.grpcTLS.Merge(&c.GRPCTLS)
	}
	f.caFile.Merge(&c.CAFile)
	f.caPath.Merge(&c.CAPath)
}

// merge the client flags into command line flags then parse command line flags
func (f *GRPCFlags) ClientFlags() *flag.FlagSet {
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	fs.Var(&f.address, "grpc-addr",
		"The `address` and `port` of the Consul GRPC agent. The value can be an IP "+
			"address or DNS address, but it must also include the port. This can also be specified "+
			"via the CONSUL_GRPC_ADDR environment variable. The default value is "+
			"127.0.0.1:8502. If you intend to communicate in TLS mode, you have to either "+
			"include https:// schema in the address, use grpc-tls flag or set environment variable "+
			"CONSUL_GRPC_TLS = true, otherwise it uses plaintext mode")
	fs.Var(&f.caFile, "grpc-tls",
		"Set to true if you aim to communicate in TLS mode in the GRPC call.")
	fs.Var(&f.caFile, "ca-file",
		"Path to a CA file to use for TLS when communicating with Consul. This "+
			"can also be specified via the CONSUL_CACERT environment variable.")
	fs.Var(&f.caPath, "ca-path",
		"Path to a directory of CA certificates to use for TLS when communicating "+
			"with Consul. This can also be specified via the CONSUL_CAPATH environment variable.")
	return fs
}
