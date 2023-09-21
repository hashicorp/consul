// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"flag"

	"github.com/hashicorp/consul/internal/resource/protoc-gen-resource-openapi/internal/generate"
	"google.golang.org/protobuf/compiler/protogen"
	plugin "google.golang.org/protobuf/types/pluginpb"
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
		files, err := generate.Generate(gp.Files)
		if err != nil {
			return err
		}

		for name, content := range files {
			out := gp.NewGeneratedFile(name, "")
			out.Write(content)
		}
		return nil
	})
}
