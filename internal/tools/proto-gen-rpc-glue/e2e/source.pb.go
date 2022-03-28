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
