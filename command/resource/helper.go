// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/anypb"
	"strings"

	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/helpers"
	"github.com/hashicorp/consul/internal/resourcehcl"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type OuterResource struct {
	ID         *ID            `json:"id"`
	Owner      *ID            `json:"owner"`
	Generation string         `json:"generation"`
	Version    string         `json:"version"`
	Metadata   map[string]any `json:"metadata"`
	Data       map[string]any `json:"data"`
}

type Tenancy struct {
	Namespace string `json:"namespace"`
	Partition string `json:"partition"`
	PeerName  string `json:"peerName"`
}
type Type struct {
	Group        string `json:"group"`
	GroupVersion string `json:"groupVersion"`
	Kind         string `json:"kind"`
}
type ID struct {
	Name    string  `json:"name"`
	Tenancy Tenancy `json:"tenancy"`
	Type    Type    `json:"type"`
	UID     string  `json:"uid"`
}

func parseJson(js string) (*pbresource.Resource, error) {

	parsedResource := new(pbresource.Resource)

	var outerResource OuterResource

	if err := json.Unmarshal([]byte(js), &outerResource); err != nil {
		return nil, err
	}

	if outerResource.ID == nil {
		return nil, fmt.Errorf("\"id\" field need to be provided")
	}

	typ := pbresource.Type{
		Kind:         outerResource.ID.Type.Kind,
		Group:        outerResource.ID.Type.Group,
		GroupVersion: outerResource.ID.Type.GroupVersion,
	}

	reg, ok := consul.NewTypeRegistry().Resolve(&typ)
	if !ok {
		return nil, fmt.Errorf("invalid type %v", parsedResource)
	}
	data := reg.Proto.ProtoReflect().New().Interface()
	anyProtoMsg, err := anypb.New(data)
	if err != nil {
		return nil, err
	}

	outerResource.Data["@type"] = anyProtoMsg.TypeUrl

	marshal, err := json.Marshal(outerResource)
	if err != nil {
		return nil, err
	}

	if err := protojson.Unmarshal(marshal, parsedResource); err != nil {
		return nil, err
	}
	return parsedResource, nil
}

func ParseResourceFromFile(filePath string) (*pbresource.Resource, error) {
	data, err := helpers.LoadDataSourceNoRaw(filePath, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to load data: %v", err)
	}
	var parsedResource *pbresource.Resource
	parsedResource, err = resourcehcl.Unmarshal([]byte(data), consul.NewTypeRegistry())
	if err != nil {
		parsedResource, err = parseJson(data)
		if err != nil {
			return nil, fmt.Errorf("Failed to decode resource from input file: %v", err)
		}
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
