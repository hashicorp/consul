// Copyright (c) HashiCorp, Inc.
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

func main() {
	flag.Parse()

	protogen.Options{
		ParamFunc: flag.CommandLine.Set,
	}.Run(func(gp *protogen.Plugin) error {
		gp.SupportedFeatures = uint64(plugin.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
		return generate.Generate(gp)
	})
}
