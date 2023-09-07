// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package apply

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"

	"github.com/mitchellh/cli"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/command/helpers"
	"github.com/hashicorp/consul/internal/resourcehcl"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func New(ui cli.Ui) *cmd {
	c := &cmd{UI: ui}
	c.init()
	return c
}

type cmd struct {
	UI    cli.Ui
	flags *flag.FlagSet
	http  *flags.HTTPFlags
	help  string

	filePath string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.filePath, "f", "",
		"File path with resource definition")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	c.help = flags.Usage(help, c.flags)
}

func makeWriteRequest(parsedResource *pbresource.Resource) (payload *api.WriteRequest, error error) {
	// The parsed hcl file has data field in proto message format anypb.Any
	// Converting to json format requires us to fisrt marshal it then unmarshal it
	data, err := protojson.Marshal(parsedResource.Data)
	if err != nil {
		return nil, fmt.Errorf("unrecognized hcl format: %s", err)
	}

	var resourceData map[string]any
	err = json.Unmarshal(data, &resourceData)
	if err != nil {
		return nil, fmt.Errorf("unrecognized hcl format: %s", err)
	}
	delete(resourceData, "@type")

	return &api.WriteRequest{
		Data:     resourceData,
		Metadata: parsedResource.GetMetadata(),
		Owner:    parsedResource.GetOwner(),
	}, nil
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		c.UI.Error(fmt.Sprintf("Failed to parse args: %v", err))
		return 1
	}

	var parsedResource *pbresource.Resource

	if c.filePath != "" {
		data, err := helpers.LoadDataSourceNoRaw(c.filePath, nil)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Failed to load data: %v", err))
			return 1
		}

		parsedResource, err = parseResource(data)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Your argument format is incorrect: %s", err))
			return 1
		}
	} else {
		c.UI.Error("Flag -f is required")
		return 1
	}

	if parsedResource == nil {
		c.UI.Error("Unable to parse the file argument")
		return 1
	}

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connect to Consul agent: %s", err))
		return 1
	}

	opts := &api.QueryOptions{
		Namespace: parsedResource.Id.Tenancy.GetNamespace(),
		Partition: parsedResource.Id.Tenancy.GetPartition(),
		Peer:      parsedResource.Id.Tenancy.GetPeerName(),
		Token:     c.http.Token(),
	}

	gvk := &api.GVK{
		Group:   parsedResource.Id.Type.GetGroup(),
		Version: parsedResource.Id.Type.GetGroupVersion(),
		Kind:    parsedResource.Id.Type.GetKind(),
	}

	writeRequest, err := makeWriteRequest(parsedResource)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error parsing hcl input: %v", err))
		return 1
	}

	entry, _, err := client.Resource().Apply(gvk, parsedResource.Id.GetName(), opts, writeRequest)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error writing resource %s/%s: %v", gvk, parsedResource.Id.GetName(), err))
		return 1
	}

	b, err := json.MarshalIndent(entry, "", "    ")
	if err != nil {
		c.UI.Error("Failed to encode output data")
		return 1
	}

	c.UI.Info(fmt.Sprintf("%s.%s.%s '%s' created.", gvk.Group, gvk.Version, gvk.Kind, parsedResource.Id.GetName()))
	c.UI.Info(string(b))
	return 0
}

func parseResource(data string) (resource *pbresource.Resource, e error) {
	// parse the data
	raw := []byte(data)
	resource, err := resourcehcl.Unmarshal(raw, consul.NewTypeRegistry())
	if err != nil {
		return nil, fmt.Errorf("Failed to decode resource from input file: %v", err)
	}

	return resource, nil
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const synopsis = "Writes/updates resource information"
const help = `
Usage: consul resource apply -f=<file-path>

Write and/or update a resource by providing the definition in an hcl file as an argument

Example:

$ consul resource apply -f=demo.hcl

Sample demo.hcl:

ID {
	Type = gvk("group.version.kind")
	Name = "resource-name"
	Tenancy {
	  Namespace = "default"
	  Partition = "default"
	  PeerName = "local"
	}
  }

  Data {
	Name = "demo"
  }

  Metadata = {
	"foo" = "bar"
  }
`
