// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource

import (
	"errors"
	"flag"
	"fmt"

	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/command/helpers"
	"github.com/hashicorp/consul/internal/resourcehcl"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func ParseResourceFromFile(filePath string) (*pbresource.Resource, error) {
	data, err := helpers.LoadDataSourceNoRaw(filePath, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to load data: %v", err)
	}
	parsedResource, err := resourcehcl.Unmarshal([]byte(data), consul.NewTypeRegistry())
	if err != nil {
		return nil, fmt.Errorf("Failed to decode resource from input file: %v", err)
	}

	return parsedResource, nil
}

func ParseInputParams(inputArgs []string, flags *flag.FlagSet) error {
	if err := flags.Parse(inputArgs); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			return fmt.Errorf("Failed to parse args: %v", err)
		}
	}
	return nil
}
