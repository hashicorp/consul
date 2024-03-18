// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package decode

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/hashicorp/consul-net-rpc/go-msgpack/codec"
	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/pbpeering"
	"github.com/hashicorp/consul/snapshot"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-raftchunking"
	"github.com/hashicorp/raft"
	"github.com/mitchellh/cli"
)

var requestTypeZeroValues = map[structs.MessageType]func() any{
	structs.RegisterRequestType:          func() any { return new(structs.RegisterRequest) },
	structs.KVSRequestType:               func() any { return new(structs.KVSRequest) },
	structs.SessionRequestType:           func() any { return new(structs.SessionRequest) },
	structs.TombstoneRequestType:         func() any { return new(structs.TombstoneRequest) },
	structs.CoordinateBatchUpdateType:    func() any { return new(structs.Coordinates) },
	structs.PreparedQueryRequestType:     func() any { return new(structs.PreparedQueryRequest) },
	structs.AutopilotRequestType:         func() any { return new(structs.AutopilotSetConfigRequest) },
	structs.IntentionRequestType:         func() any { return new(structs.IntentionRequest) },
	structs.ConnectCARequestType:         func() any { return new(structs.CARequest) },
	structs.ConnectCAProviderStateType:   func() any { return new(structs.CAConsulProviderState) },
	structs.ConnectCAConfigType:          func() any { return new(structs.CAConfiguration) },
	structs.IndexRequestType:             func() any { return new(state.IndexEntry) },
	structs.ACLTokenSetRequestType:       func() any { return new(structs.ACLToken) },
	structs.ACLPolicySetRequestType:      func() any { return new(structs.ACLPolicy) },
	structs.ConfigEntryRequestType:       func() any { return new(structs.ConfigEntryRequest) },
	structs.ACLRoleSetRequestType:        func() any { return new(structs.ACLRole) },
	structs.ACLBindingRuleSetRequestType: func() any { return new(structs.ACLBindingRule) },
	structs.ACLAuthMethodSetRequestType:  func() any { return new(structs.ACLAuthMethod) },
	structs.ChunkingStateType:            func() any { return new(raftchunking.State) },
	structs.FederationStateRequestType:   func() any { return new(structs.FederationStateRequest) },
	structs.SystemMetadataRequestType:    func() any { return new(structs.SystemMetadataEntry) },
	structs.ServiceVirtualIPRequestType:  func() any { return new(state.ServiceVirtualIP) },
	structs.FreeVirtualIPRequestType:     func() any { return new(state.FreeVirtualIP) },
	structs.PeeringWriteType:             func() any { return new(pbpeering.Peering) },
	structs.PeeringTrustBundleWriteType:  func() any { return new(pbpeering.PeeringTrustBundle) },
	structs.PeeringSecretsWriteType:      func() any { return new(pbpeering.PeeringSecrets) },
	structs.ResourceOperationType:        func() any { return new(pbresource.Resource) },
}

func New(ui cli.Ui) *cmd {
	c := &cmd{UI: ui}
	c.init()
	return c
}

type cmd struct {
	UI     cli.Ui
	flags  *flag.FlagSet
	help   string
	format string

	encoder *json.Encoder
}

func (c *cmd) Write(p []byte) (n int, err error) {
	s := string(p)
	c.UI.Output(strings.TrimRight(s, "\n"))
	return len(s), nil
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.help = flags.Usage(help, c.flags)
	c.encoder = json.NewEncoder(c)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	var file string
	args = c.flags.Args()

	switch len(args) {
	case 0:
		c.UI.Error("Missing FILE argument")
		return 1
	case 1:
		file = args[0]
	default:
		c.UI.Error(fmt.Sprintf("Too many arguments (expected 1, got %d)", len(args)))
		return 1
	}

	// Open the file.
	f, err := os.Open(file)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error opening snapshot file: %s", err))
		return 1
	}
	defer f.Close()

	var readFile *os.File
	var meta *raft.SnapshotMeta

	if strings.ToLower(path.Base(file)) == "state.bin" {
		// This is an internal raw raft snapshot not a gzipped archive one
		// downloaded from the API, we can read it directly
		readFile = f

		// Assume the meta is colocated and error if not.
		metaRaw, err := os.ReadFile(path.Join(path.Dir(file), "meta.json"))
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error reading meta.json from internal snapshot dir: %s", err))
			return 1
		}
		var metaDecoded raft.SnapshotMeta
		err = json.Unmarshal(metaRaw, &metaDecoded)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error parsing meta.json from internal snapshot dir: %s", err))
			return 1
		}
		meta = &metaDecoded
	} else {
		readFile, meta, err = snapshot.Read(hclog.New(nil), f)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error reading snapshot: %s", err))
			return 1
		}
		defer func() {
			if err := readFile.Close(); err != nil {
				c.UI.Error(fmt.Sprintf("Failed to close temp snapshot: %v", err))
			}
			if err := os.Remove(readFile.Name()); err != nil {
				c.UI.Error(fmt.Sprintf("Failed to clean up temp snapshot: %v", err))
			}
		}()
	}

	err = c.encoder.Encode(map[string]interface{}{
		"Type": "SnapshotHeader",
		"Data": meta,
	})
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error encoding snapshot header to the output stream: %v", err))
		return 1
	}

	err = c.decodeStream(readFile)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error extracting snapshot data: %s", err))
		return 1
	}

	return 0
}

// enhance utilizes ReadSnapshot to populate the struct with
// all of the snapshot's itemized data
func (c *cmd) decodeStream(file io.Reader) error {
	handler := func(header *fsm.SnapshotHeader, msg structs.MessageType, dec *codec.Decoder) error {
		name := structs.MessageType.String(msg)
		var val interface{}

		if zeroVal, ok := requestTypeZeroValues[msg]; ok {
			val = zeroVal()
		}

		err := dec.Decode(&val)
		if err != nil {
			return fmt.Errorf("failed to decode msg type %v, error %v", name, err)
		}

		err = c.encoder.Encode(map[string]interface{}{
			"Type": name,
			"Data": val,
		})

		if err != nil {
			return fmt.Errorf("failed to encode data into the object stream: %w", err)
		}

		return nil
	}

	return fsm.ReadSnapshot(file, handler)
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Decodes the binary"
const help = `
Usage: consul snapshot decode [options] FILE

  Decodes snapshot data and outputs a stream of line delimited JSON objects of the form:
  
  {"Type": "SnapshotHeader", "Data": {"<json encoded snapshot header>"}}
  {"Type": "<type name>","Data": {<json encoded data>}}
  {"Type": "<type name>","Data": {<json encoded data>}}
  ...
`
