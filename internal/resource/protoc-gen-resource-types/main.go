// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"flag"

	"github.com/hashicorp/consul/internal/resource/protoc-gen-resource-types/internal/generate"
	"google.golang.org/protobuf/compiler/protogen"
	plugin "google.golang.org/protobuf/types/pluginpb"
)

var (
	prefix = flag.String("prefix", "resource_types.gen", "filename prefix to use. '.go' will be appended to this to create the full filename")
)

func main() {
	flag.Parse()

	protogen.Options{
		ParamFunc: flag.CommandLine.Set,
	}.Run(func(gp *protogen.Plugin) error {
		gp.SupportedFeatures = uint64(plugin.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
		return generate.Generate(gp, *prefix)
	})
}
