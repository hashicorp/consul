// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import "flag"

type ResourceFlags struct {
	namespace TValue[string]
	partition TValue[string]
	peername  TValue[string]
	stale     TValue[bool]
}

func (f *ResourceFlags) ResourceFlags() *flag.FlagSet {
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	fs.Var(&f.namespace, "namespace",
		"Specifies the namespace to query. It will default to the `default` namespace.`")
	fs.Var(&f.partition, "partition",
		"Specifies the admin partition to query. If not provided, the admin partition will be inferred "+
			"from the request's ACL token, or will default to the `default` admin partition. "+
			"Admin Partitions are a Consul Enterprise feature.")
	fs.Var(&f.peername, "peer", "Specifies the name of peer to query. By default, it is `local`.")
	fs.Var(&f.stale, "stale",
		"Permit any Consul server (non-leader) to respond to this request. This "+
			"allows for lower latency and higher throughput, but can result in "+
			"stale data. This option has no effect on non-read operations. The "+
			"default value is false.")
	return fs
}

func (f *ResourceFlags) Namespace() string {
	return f.namespace.String()
}

func (f *ResourceFlags) Partition() string {
	return f.partition.String()
}

func (f *ResourceFlags) Peername() string {
	return f.peername.String()
}

func (f *ResourceFlags) Stale() bool {
	if f.stale.v == nil {
		return false
	}
	return *f.stale.v
}
