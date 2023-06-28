// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package flags

import (
	"flag"
	"os"
	"strings"

	"github.com/hashicorp/consul/api"
)

type HTTPFlags struct {
	// client api flags
	address       StringValue
	token         StringValue
	tokenFile     StringValue
	caFile        StringValue
	caPath        StringValue
	certFile      StringValue
	keyFile       StringValue
	tlsServerName StringValue

	// server flags
	datacenter StringValue
	stale      BoolValue

	// multi-tenancy flags
	namespace StringValue
	partition StringValue
}

func (f *HTTPFlags) ClientFlags() *flag.FlagSet {
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	fs.Var(&f.address, "http-addr",
		"The `address` and port of the Consul HTTP agent. The value can be an IP "+
			"address or DNS address, but it must also include the port. This can "+
			"also be specified via the CONSUL_HTTP_ADDR environment variable. The "+
			"default value is http://127.0.0.1:8500. The scheme can also be set to "+
			"HTTPS by setting the environment variable CONSUL_HTTP_SSL=true.")
	fs.Var(&f.token, "token",
		"ACL token to use in the request. This can also be specified via the "+
			"CONSUL_HTTP_TOKEN environment variable. If unspecified, the query will "+
			"default to the token of the Consul agent at the HTTP address.")
	fs.Var(&f.tokenFile, "token-file",
		"File containing the ACL token to use in the request instead of one specified "+
			"via the -token argument or CONSUL_HTTP_TOKEN environment variable. "+
			"This can also be specified via the CONSUL_HTTP_TOKEN_FILE environment variable.")
	fs.Var(&f.caFile, "ca-file",
		"Path to a CA file to use for TLS when communicating with Consul. This "+
			"can also be specified via the CONSUL_CACERT environment variable.")
	fs.Var(&f.caPath, "ca-path",
		"Path to a directory of CA certificates to use for TLS when communicating "+
			"with Consul. This can also be specified via the CONSUL_CAPATH environment variable.")
	fs.Var(&f.certFile, "client-cert",
		"Path to a client cert file to use for TLS when 'verify_incoming' is enabled. This "+
			"can also be specified via the CONSUL_CLIENT_CERT environment variable.")
	fs.Var(&f.keyFile, "client-key",
		"Path to a client key file to use for TLS when 'verify_incoming' is enabled. This "+
			"can also be specified via the CONSUL_CLIENT_KEY environment variable.")
	fs.Var(&f.tlsServerName, "tls-server-name",
		"The server name to use as the SNI host when connecting via TLS. This "+
			"can also be specified via the CONSUL_TLS_SERVER_NAME environment variable.")
	return fs
}

func (f *HTTPFlags) ServerFlags() *flag.FlagSet {
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	fs.Var(&f.datacenter, "datacenter",
		"Name of the datacenter to query. If unspecified, this will default to "+
			"the datacenter of the queried agent.")
	fs.Var(&f.stale, "stale",
		"Permit any Consul server (non-leader) to respond to this request. This "+
			"allows for lower latency and higher throughput, but can result in "+
			"stale data. This option has no effect on non-read operations. The "+
			"default value is false.")
	return fs
}

func (f *HTTPFlags) MultiTenancyFlags() *flag.FlagSet {
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	fs.Var(&f.namespace, "namespace",
		"Specifies the namespace to query. If not provided, the namespace will be inferred "+
			"from the request's ACL token, or will default to the `default` namespace. "+
			"Namespaces are a Consul Enterprise feature.")
	f.AddPartitionFlag(fs)
	return fs
}

func (f *HTTPFlags) PartitionFlag() *flag.FlagSet {
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	f.AddPartitionFlag(fs)
	return fs
}
func (f *HTTPFlags) Addr() string {
	return f.address.String()
}

func (f *HTTPFlags) Datacenter() string {
	return f.datacenter.String()
}

func (f *HTTPFlags) Namespace() string {
	return f.namespace.String()
}

func (f *HTTPFlags) Partition() string {
	return f.partition.String()
}

func (f *HTTPFlags) Stale() bool {
	if f.stale.v == nil {
		return false
	}
	return *f.stale.v
}

func (f *HTTPFlags) Token() string {
	return f.token.String()
}

func (f *HTTPFlags) SetToken(v string) error {
	return f.token.Set(v)
}

func (f *HTTPFlags) TokenFile() string {
	return f.tokenFile.String()
}

func (f *HTTPFlags) SetTokenFile(v string) error {
	return f.tokenFile.Set(v)
}

func (f *HTTPFlags) ReadTokenFile() (string, error) {
	tokenFile := f.tokenFile.String()
	if tokenFile == "" {
		return "", nil
	}

	data, err := os.ReadFile(tokenFile)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(data)), nil
}

func (f *HTTPFlags) APIClient() (*api.Client, error) {
	c := api.DefaultConfig()

	f.MergeOntoConfig(c)

	return api.NewClient(c)
}

func (f *HTTPFlags) MergeOntoConfig(c *api.Config) {
	f.address.Merge(&c.Address)
	f.token.Merge(&c.Token)
	f.tokenFile.Merge(&c.TokenFile)
	f.caFile.Merge(&c.TLSConfig.CAFile)
	f.caPath.Merge(&c.TLSConfig.CAPath)
	f.certFile.Merge(&c.TLSConfig.CertFile)
	f.keyFile.Merge(&c.TLSConfig.KeyFile)
	f.tlsServerName.Merge(&c.TLSConfig.Address)
	f.datacenter.Merge(&c.Datacenter)
	f.namespace.Merge(&c.Namespace)
	f.partition.Merge(&c.Partition)
}

func (f *HTTPFlags) AddPartitionFlag(fs *flag.FlagSet) {
	fs.Var(&f.partition, "partition",
		"Specifies the admin partition to query. If not provided, the admin partition will be inferred "+
			"from the request's ACL token, or will default to the `default` admin partition. "+
			"Admin Partitions are a Consul Enterprise feature.")
}
