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
	"net/http"
	"strings"

	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/command/helpers"
	"github.com/hashicorp/consul/command/resource/client"
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

func GetTypeAndResourceName(args []string) (gvk *GVK, resourceName string, e error) {
	// it has to be resource name after the type
	if strings.HasPrefix(args[1], "-") {
		return nil, "", fmt.Errorf("Must provide resource name right after type")
	}

	s := strings.Split(args[0], ".")
	if len(s) != 3 {
		return nil, "", fmt.Errorf("Must include resource type argument in group.verion.kind format")
	}

	gvk = &GVK{
		Group:   s[0],
		Version: s[1],
		Kind:    s[2],
	}

	resourceName = args[1]
	return
}

type Resource struct {
	C *client.Client
}

type GVK struct {
	Group   string
	Version string
	Kind    string
}

type WriteRequest struct {
	Metadata map[string]string `json:"metadata"`
	Data     map[string]any    `json:"data"`
	Owner    *pbresource.ID    `json:"owner"`
}

type ListResponse struct {
	Resources []map[string]interface{} `json:"resources"`
}

func (resource *Resource) Read(gvk *GVK, resourceName string, q *client.QueryOptions) (map[string]interface{}, error) {
	r := resource.C.NewRequest("GET", strings.ToLower(fmt.Sprintf("/api/%s/%s/%s/%s", gvk.Group, gvk.Version, gvk.Kind, resourceName)))
	r.SetQueryOptions(q)
	_, resp, err := resource.C.DoRequest(r)
	if err != nil {
		return nil, err
	}
	defer client.CloseResponseBody(resp)
	if err := client.RequireOK(resp); err != nil {
		return nil, err
	}

	var out map[string]interface{}
	if err := client.DecodeBody(resp, &out); err != nil {
		return nil, err
	}

	return out, nil
}

func (resource *Resource) Delete(gvk *GVK, resourceName string, q *client.QueryOptions) error {
	r := resource.C.NewRequest("DELETE", strings.ToLower(fmt.Sprintf("/api/%s/%s/%s/%s", gvk.Group, gvk.Version, gvk.Kind, resourceName)))
	r.SetQueryOptions(q)
	_, resp, err := resource.C.DoRequest(r)
	if err != nil {
		return err
	}
	defer client.CloseResponseBody(resp)
	if err := client.RequireHttpCodes(resp, http.StatusNoContent); err != nil {
		return err
	}
	return nil
}

func (resource *Resource) Apply(gvk *GVK, resourceName string, q *client.QueryOptions, payload *WriteRequest) (*map[string]interface{}, error) {
	url := strings.ToLower(fmt.Sprintf("/api/%s/%s/%s/%s", gvk.Group, gvk.Version, gvk.Kind, resourceName))

	r := resource.C.NewRequest("PUT", url)
	r.SetQueryOptions(q)
	r.Obj = payload
	_, resp, err := resource.C.DoRequest(r)
	if err != nil {
		return nil, err
	}
	defer client.CloseResponseBody(resp)
	if err := client.RequireOK(resp); err != nil {
		return nil, err
	}

	var out map[string]interface{}

	if err := client.DecodeBody(resp, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

func (resource *Resource) List(gvk *GVK, q *client.QueryOptions) (*ListResponse, error) {
	r := resource.C.NewRequest("GET", strings.ToLower(fmt.Sprintf("/api/%s/%s/%s", gvk.Group, gvk.Version, gvk.Kind)))
	r.SetQueryOptions(q)
	_, resp, err := resource.C.DoRequest(r)
	if err != nil {
		return nil, err
	}
	defer client.CloseResponseBody(resp)
	if err := client.RequireOK(resp); err != nil {
		return nil, err
	}

	var out *ListResponse
	if err := client.DecodeBody(resp, &out); err != nil {
		return nil, err
	}

	return out, nil
}
