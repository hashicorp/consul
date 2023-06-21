// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// protoc-gen-consul-rate-limit
// This protoc plugin maintains the mapping of gRPC method names to
// a specification of how they should be rate-limited. This is used by the gRPC
// InTapHandle function (see agent/grpc-middleware/rate.go) to enforce relevant
// limits without having to call the handler. It works in two phases:
//
// # Phase 1
//
//	Buf/protoc invokes this plugin for each .proto file. We extract the rate limit
//	specification from an annotation on the RPC:
//
//		service Foo {
//			rpc Bar(BarRequest) returns (BarResponse) {
//				option (hashicorp.consul.internal.ratelimit.spec) = {
//					operation_type: OPERATION_TYPE_WRITE,
//					operation_category: OPERATION_CATEGORY_ACL
//				};
//			}
//		}
//
// We write a JSON array of the limits to protobuf/package/path/.ratelimit.tmp:
//
//	[
//		{
//			"MethodName": "/Foo/Bar",
//			"OperationType": "OPERATION_TYPE_WRITE",
//			"OperationCategory": "OPERATION_CATEGORY_ACL"
//		}
//	]
//
// # Phase 2
//
// The protobuf.sh script (invoked by make proto) runs our postprocess script
// which reads all of the .ratelimit.tmp files in proto and proto-public and
// generates a single Go map in agent/grpc-middleware/rate_limit_mappings.gen.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/proto-public/annotations/ratelimit"
)

const (
	outputFileName = ".ratelimit.tmp"

	missingSpecTmpl = `RPC %s is missing rate-limit specification, fix it with:

	import "proto-public/annotations/ratelimit/ratelimit.proto";

	service %s {
	  rpc %s(...) returns (...) {
	    option (hashicorp.consul.internal.ratelimit.spec) = {
	      operation_type: OPERATION_TYPE_READ | OPERATION_TYPE_WRITE | OPERATION_TYPE_EXEMPT,
		  operation_category: OPERATION_CATEGORY_ACL | OPERATION_CATEGORY_PEER_STREAM | OPERATION_CATEGORY_CONNECT_CA | OPERATION_CATEGORY_PARTITION | OPERATION_CATEGORY_PEERING | OPERATION_CATEGORY_SERVER_DISCOVERY | OPERATION_CATEGORY_DATAPLANE | OPERATION_CATEGORY_DNS | OPERATION_CATEGORY_SUBSCRIBE | OPERATION_CATEGORY_OPERATOR | OPERATION_CATEGORY_RESOURCE,
	    };
	  }
	}
`

	enterpriseBuildTag = "//go:build consulent"
)

type rateLimitSpec struct {
	MethodName        string
	OperationType     string
	OperationCategory string
	Enterprise        bool
}

func main() {
	var opts protogen.Options
	opts.Run(func(plugin *protogen.Plugin) error {
		for _, path := range plugin.Request.FileToGenerate {
			file, ok := plugin.FilesByPath[path]
			if !ok {
				return fmt.Errorf("failed to get file descriptor: %s", path)
			}

			specs, err := rateLimitSpecs(file)
			if err != nil {
				return err
			}

			if len(specs) == 0 {
				return nil
			}

			outputPath := filepath.Join(filepath.Dir(path), outputFileName)
			output := plugin.NewGeneratedFile(outputPath, "")
			if err := json.NewEncoder(output).Encode(specs); err != nil {
				return err
			}
		}
		return nil
	})
}

func rateLimitSpecs(file *protogen.File) ([]rateLimitSpec, error) {
	enterprise, err := isEnterpriseFile(file)
	if err != nil {
		return nil, err
	}

	var specs []rateLimitSpec
	for _, service := range file.Services {
		for _, method := range service.Methods {
			spec := rateLimitSpec{
				// Format the method name in gRPC/HTTP path format.
				MethodName: fmt.Sprintf("/%s/%s", service.Desc.FullName(), method.Desc.Name()),
				Enterprise: enterprise,
			}

			// Read the rate limit spec from the method options.
			options := method.Desc.Options()
			if !proto.HasExtension(options, ratelimit.E_Spec) {
				err := fmt.Errorf(missingSpecTmpl,
					method.Desc.Name(),
					service.Desc.Name(),
					method.Desc.Name())
				return nil, err
			}

			def := proto.GetExtension(options, ratelimit.E_Spec).(*ratelimit.Spec)
			spec.OperationType = def.OperationType.String()
			spec.OperationCategory = def.OperationCategory.String()

			specs = append(specs, spec)
		}
	}
	return specs, nil
}

func isEnterpriseFile(file *protogen.File) (bool, error) {
	source, err := os.ReadFile(file.Desc.Path())
	if err != nil {
		return false, fmt.Errorf("failed to read proto file: %w", err)
	}
	return bytes.Contains(source, []byte(enterpriseBuildTag)), nil
}
