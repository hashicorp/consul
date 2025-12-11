// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"flag"

	"google.golang.org/protobuf/compiler/protogen"
	plugin "google.golang.org/protobuf/types/pluginpb"

	"github.com/hashicorp/consul/internal/resource/protoc-gen-json-shim/internal/generate"
)

var (
	file = flag.String("file", "-", "where to load data from")
)

// This file is responsible for generating a JSON marhsal/unmarshal overwrite for proto
// structs which allows Kubernetes CRDs to get created directly from the proto-types.
func main() {
	flag.Parse()

	protogen.Options{
		ParamFunc: flag.CommandLine.Set,
	}.Run(func(gp *protogen.Plugin) error {
		gp.SupportedFeatures = uint64(plugin.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
		return generate.Generate(gp)
	})
}
