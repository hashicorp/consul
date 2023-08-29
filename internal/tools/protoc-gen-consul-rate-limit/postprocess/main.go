// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	usage = "Usage: %s -input=/proto-dir-1 -input=/proto-dir-2 -output=/mappings.go\n"

	fileHeader = `// generated by protoc-gen-consul-rate-limit; DO NOT EDIT.
package middleware

import "github.com/hashicorp/consul/agent/consul/rate"
`

	entTags = `//go:build consulent
// +build consulent
`
)

func main() {
	var (
		inputPaths sliceFlags
		outputPath string
	)
	flag.Var(&inputPaths, "input", "")
	flag.StringVar(&outputPath, "output", "", "")
	flag.Parse()

	if len(inputPaths) == 0 || outputPath == "" {
		fmt.Fprintf(os.Stderr, usage, os.Args[0])
		os.Exit(1)
	}

	if err := run(inputPaths, outputPath); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func run(inputPaths []string, outputPath string) error {
	if !strings.HasSuffix(outputPath, ".go") {
		return errors.New("-output path must end in .go")
	}

	ce, ent, err := collectSpecs(inputPaths)
	if err != nil {
		return err
	}

	ceSource, err := generateCE(ce)
	if err != nil {
		return err
	}
	if err := os.WriteFile(outputPath, ceSource, 0666); err != nil {
		return fmt.Errorf("failed to write output file: %s - %w", outputPath, err)
	}

	// ent should only be non-zero in the enterprise repository.
	if len(ent) > 0 {
		entSource, err := generateENT(ent)
		if err != nil {
			return err
		}

		if err := os.WriteFile(enterpriseFileName(outputPath), entSource, 0666); err != nil {
			return fmt.Errorf("failed to write output file: %s - %w", outputPath, err)
		}
	}

	return nil
}

// enterpriseFileName adds the _ent filename suffix before the extension.
//
// Example:
//
//	enterpriseFileName("bar/baz/foo.gen.go") => "bar/baz/foo_ent.gen.go"
func enterpriseFileName(filename string) string {
	fileName := filepath.Base(filename)
	extStart := strings.Index(fileName, ".")
	return filepath.Join(
		filepath.Dir(filename),
		fileName[0:extStart]+"_ent"+fileName[extStart:],
	)
}

type spec struct {
	MethodName        string
	OperationType     string
	Enterprise        bool
	OperationCategory string
}

func (s spec) GoOperationType() string {
	switch s.OperationType {
	case "OPERATION_TYPE_WRITE":
		return "rate.OperationTypeWrite"
	case "OPERATION_TYPE_READ":
		return "rate.OperationTypeRead"
	case "OPERATION_TYPE_EXEMPT":
		return "rate.OperationTypeExempt"
	}
	panic(fmt.Sprintf("unknown rate limit operation type: %s", s.OperationType))
}

func (s spec) GoOperationCategory() string {
	switch s.OperationCategory {
	case "OPERATION_CATEGORY_ACL":
		return "rate.OperationCategoryACL"
	case "OPERATION_CATEGORY_PEER_STREAM":
		return "rate.OperationCategoryPeerStream"
	case "OPERATION_CATEGORY_CONNECT_CA":
		return "rate.OperationCategoryConnectCA"
	case "OPERATION_CATEGORY_PARTITION":
		return "rate.OperationCategoryPartition"
	case "OPERATION_CATEGORY_PEERING":
		return "rate.OperationCategoryPeering"
	case "OPERATION_CATEGORY_SERVER_DISCOVERY":
		return "rate.OperationCategoryServerDiscovery"
	case "OPERATION_CATEGORY_DATAPLANE":
		return "rate.OperationCategoryDataPlane"
	case "OPERATION_CATEGORY_DNS":
		return "rate.OperationCategoryDNS"
	case "OPERATION_CATEGORY_SUBSCRIBE":
		return "rate.OperationCategorySubscribe"
	case "OPERATION_CATEGORY_OPERATOR":
		return "rate.OperationCategoryOperator"
	case "OPERATION_CATEGORY_RESOURCE":
		return "rate.OperationCategoryResource"
	}
	panic(fmt.Sprintf("unknown rate limit operation category: %s found in method: %s", s.OperationCategory, s.MethodName))
}

func collectSpecs(inputPaths []string) ([]spec, []spec, error) {
	var specs []spec
	var specFiles []string
	for _, protoPath := range inputPaths {
		err := filepath.WalkDir(protoPath, func(path string, info fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if info.Name() == ".ratelimit.tmp" {
				specFiles = append(specFiles, path)
			}

			return nil
		})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to walk directory: %s - %w", protoPath, err)
		}
	}

	for _, file := range specFiles {
		b, err := os.ReadFile(file)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read ratelimit file: %w", err)
		}

		var fileSpecs []spec
		if err := json.Unmarshal(b, &fileSpecs); err != nil {
			return nil, nil, fmt.Errorf("failed to unmarshal ratelimit file %s - %w", file, err)
		}
		specs = append(specs, fileSpecs...)
	}

	sort.Slice(specs, func(a, b int) bool {
		return specs[a].MethodName < specs[b].MethodName
	})

	var ce, ent []spec
	for _, spec := range specs {
		if spec.Enterprise {
			ent = append(ent, spec)
		} else {
			ce = append(ce, spec)
		}
	}

	return ce, ent, nil
}

func generateCE(specs []spec) ([]byte, error) {
	var output bytes.Buffer
	output.WriteString(fileHeader)

	fmt.Fprintln(&output, `var rpcRateLimitSpecs = map[string]rate.OperationSpec{`)
	for _, spec := range specs {
		fmt.Fprintf(&output, `"%s": {Type: %s, Category: %s},`, spec.MethodName, spec.GoOperationType(), spec.GoOperationCategory())
		output.WriteString("\n")
	}
	output.WriteString("}")

	formatted, err := format.Source(output.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to format source in ce: %w", err)
	}
	return formatted, nil
}

func generateENT(specs []spec) ([]byte, error) {
	var output bytes.Buffer
	output.WriteString(entTags)
	output.WriteString(fileHeader)

	output.WriteString("func init() {\n")
	for _, spec := range specs {
		fmt.Fprintf(&output, `rpcRateLimitSpecs["%s"] = rate.OperationSpec{Type: %s, Category: %s}`, spec.MethodName, spec.GoOperationType(), spec.GoOperationCategory())
		output.WriteString("\n")
	}
	output.WriteString("}")

	formatted, err := format.Source(output.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to format source in ent: %w", err)
	}
	return formatted, nil
}

type sliceFlags []string

func (i *sliceFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func (i *sliceFlags) String() string { return "" }
