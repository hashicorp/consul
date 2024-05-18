// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"flag"
	"strings"
)

type GRPCFlags struct {
	address   TValue[string]
	grpcTLS   TValue[bool]
	certFile  TValue[string]
	keyFile   TValue[string]
	caFile    TValue[string]
	caPath    TValue[string]
	token     TValue[string]
	tokenFile TValue[string]
}

// MergeFlagsIntoGRPCConfig merges flag values into grpc config
// caller has to parse the CLI args before loading them into flag values
// The flags take precedence over the environment values
func (f *GRPCFlags) MergeFlagsIntoGRPCConfig(c *GRPCConfig) {
	if strings.HasPrefix(strings.ToLower(f.address.String()), "https://") {
		c.GRPCTLS = true
	}
	if f.address.v != nil {
		f.address.Set(removeSchemaFromGRPCAddress(f.address.String()))
		f.address.Merge(&c.Address)
	}
	// won't overwrite the value if it's false
	if f.grpcTLS.v != nil && *f.grpcTLS.v {
		f.grpcTLS.Merge(&c.GRPCTLS)
	}
	f.certFile.Merge(&c.CertFile)
	f.keyFile.Merge(&c.KeyFile)
	f.caFile.Merge(&c.CAFile)
	f.caPath.Merge(&c.CAPath)
	f.token.Merge(&c.Token)
	f.tokenFile.Merge(&c.TokenFile)
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
	fs.Var(&f.grpcTLS, "grpc-tls",
		"Set to true if you aim to communicate in TLS mode in the GRPC call.")
	fs.Var(&f.certFile, "client-cert",
		"Path to a client cert file to use for TLS when 'verify_incoming' is enabled. This "+
			"can also be specified via the CONSUL_GRPC_CLIENT_CERT environment variable.")
	fs.Var(&f.keyFile, "client-key",
		"Path to a client key file to use for TLS when 'verify_incoming' is enabled. This "+
			"can also be specified via the CONSUL_GRPC_CLIENT_KEY environment variable.")
	fs.Var(&f.caFile, "ca-file",
		"Path to a CA file to use for TLS when communicating with Consul. This "+
			"can also be specified via the CONSUL_CACERT environment variable.")
	fs.Var(&f.caPath, "ca-path",
		"Path to a directory of CA certificates to use for TLS when communicating "+
			"with Consul. This can also be specified via the CONSUL_CAPATH environment variable.")
	fs.Var(&f.token, "token",
		"ACL token to use in the request. This can also be specified via the "+
			"CONSUL_GRPC_TOKEN environment variable. If unspecified, the query will "+
			"default to the token of the Consul agent at the GRPC address.")
	fs.Var(&f.tokenFile, "token-file",
		"File containing the ACL token to use in the request instead of one specified "+
			"via the -token-file argument or CONSUL_GRPC_TOKEN_FILE environment variable. "+
			"Notice the tokenFile takes precedence over token flag and environment variables.")
	return fs
}

func MergeFlags(dst, src *flag.FlagSet) {
	if dst == nil {
		panic("dst cannot be nil")
	}
	if src == nil {
		return
	}
	src.VisitAll(func(f *flag.Flag) {
		dst.Var(f.Value, f.Name, f.Usage)
	})
}
