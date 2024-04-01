// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package apply

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/mitchellh/cli"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/command/resource"
	"github.com/hashicorp/consul/command/resource/client"
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

	testStdin io.Reader
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.filePath, "f", "",
		"File path with resource definition")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	c.help = flags.Usage(help, c.flags)
}

func makeWriteRequest(parsedResource *pbresource.Resource) (payload *resource.WriteRequest, error error) {
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

	return &resource.WriteRequest{
		Data:     resourceData,
		Metadata: parsedResource.GetMetadata(),
		Owner:    parsedResource.GetOwner(),
	}, nil
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			c.UI.Error(fmt.Sprintf("Failed to parse args: %v", err))
			return 1
		}
	}

	input := c.filePath

	if input == "" && len(c.flags.Args()) > 0 {
		input = c.flags.Arg(0)
	}

	var parsedResource *pbresource.Resource

	if input != "" {
		data, err := resource.ParseResourceInput(input, c.testStdin)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Failed to decode resource from input file: %v", err))
			return 1
		}
		parsedResource = data
	} else {
		c.UI.Error("Incorrect argument format: Must provide exactly one positional argument to specify the resource to write")
		return 1
	}

	if parsedResource == nil {
		c.UI.Error("Unable to parse the file argument")
		return 1
	}

	config := api.DefaultConfig()

	c.http.MergeOntoConfig(config)
	resourceClient, err := client.NewClient(config)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connect to Consul agent: %s", err))
		return 1
	}

	res := resource.Resource{C: resourceClient}

	opts := &client.QueryOptions{
		Namespace: parsedResource.Id.Tenancy.GetNamespace(),
		Partition: parsedResource.Id.Tenancy.GetPartition(),
		Token:     c.http.Token(),
	}

	gvk := &resource.GVK{
		Group:   parsedResource.Id.Type.GetGroup(),
		Version: parsedResource.Id.Type.GetGroupVersion(),
		Kind:    parsedResource.Id.Type.GetKind(),
	}

	writeRequest, err := makeWriteRequest(parsedResource)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error parsing hcl input: %v", err))
		return 1
	}

	entry, err := res.Apply(gvk, parsedResource.Id.GetName(), opts, writeRequest)
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

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const synopsis = "Writes/updates resource information"

const help = `
Usage: consul resource apply [options] <resource>

	Write and/or update a resource by providing the definition. The configuration
	argument is either a file path or '-' to indicate that the resource
    should be read from stdin. The data should be either in HCL or
	JSON form.

	Example (with flag):

	$ consul resource apply -f=demo.hcl

	Example (from file):

	$ consul resource apply demo.hcl

	Example (from stdin):

	$ consul resource apply -

	Sample demo.hcl:

	ID {
		Type = gvk("group.version.kind")
		Name = "resource-name"
		Tenancy {
		Namespace = "default"
		Partition = "default"
		}
	}

	Data {
		Name = "demo"
	}

	Metadata = {
		"foo" = "bar"
	}
`
