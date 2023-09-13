// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/api"
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

func GetTypeAndResourceName(args []string) (gvk *api.GVK, resourceName string, e error) {
	// it has to be resource name after the type
	if strings.HasPrefix(args[1], "-") {
		return nil, "", fmt.Errorf("Must provide resource name right after type")
	}

	s := strings.Split(args[0], ".")
	if len(s) != 3 {
		return nil, "", fmt.Errorf("Must include resource type argument in group.verion.kind format")
	}

	gvk = &api.GVK{
		Group:   s[0],
		Version: s[1],
		Kind:    s[2],
	}

	resourceName = args[1]
	return
}
