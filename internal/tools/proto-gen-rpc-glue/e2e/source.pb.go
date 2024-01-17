//go:build example
// +build example

package e2e

import "github.com/hashicorp/consul/proto/pbcommon"

// @consul-rpc-glue: WriteRequest,TargetDatacenter
type ExampleWriteRequest struct {
	Value            string
	WriteRequest     *pbcommon.WriteRequest
	TargetDatacenter *pbcommon.TargetDatacenter
}

// @consul-rpc-glue: ReadRequest,TargetDatacenter
type ExampleReadRequest struct {
	Value            string
	ReadRequest      *pbcommon.ReadRequest
	TargetDatacenter *pbcommon.TargetDatacenter
}

// @consul-rpc-glue: QueryOptions,TargetDatacenter
type ExampleQueryOptions struct {
	Value            string
	QueryOptions     *pbcommon.QueryOptions
	TargetDatacenter *pbcommon.TargetDatacenter
}

// @consul-rpc-glue: QueryMeta
type ExampleQueryMeta struct {
	Value     string
	QueryMeta *pbcommon.QueryMeta
}

// @consul-rpc-glue: Datacenter
type ExampleDatacenter struct {
	Value      string
	Datacenter string
}

// @consul-rpc-glue: WriteRequest=AltWriteRequest
type AltExampleWriteRequest struct {
	Value           int
	AltWriteRequest *pbcommon.WriteRequest
}

// @consul-rpc-glue: ReadRequest=AltReadRequest
type AltExampleReadRequest struct {
	Value          int
	AltReadRequest *pbcommon.ReadRequest
}

// @consul-rpc-glue: QueryOptions=AltQueryOptions
type AltExampleQueryOptions struct {
	Value           string
	AltQueryOptions *pbcommon.QueryOptions
}

// @consul-rpc-glue: QueryMeta=AltQueryMeta
type AltExampleQueryMeta struct {
	AltQueryMeta *pbcommon.QueryMeta
}

// @consul-rpc-glue: Datacenter=AltDatacenter
type AltExampleDatacenter struct {
	Value         string
	AltDatacenter string
}
