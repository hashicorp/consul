// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/hashicorp/hcl"
	"github.com/mitchellh/mapstructure"
	"github.com/testcontainers/testcontainers-go"
	"google.golang.org/grpc"

	agentconfig "github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib/decode"

	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

// Agent represent a Consul agent abstraction
type Agent interface {
	GetIP() string
	GetClient() *api.Client
	NewClient(string, bool) (*api.Client, error)
	GetName() string
	GetAgentName() string
	GetPartition() string
	GetPod() testcontainers.Container
	Logs(context.Context) (io.ReadCloser, error)
	ClaimAdminPort() (int, error)
	GetConfig() Config
	GetInfo() AgentInfo
	GetDatacenter() string
	GetNetwork() string
	IsServer() bool
	RegisterTermination(func() error)
	Terminate() error
	TerminateAndRetainPod(bool) error
	Upgrade(ctx context.Context, config Config) error
	Exec(ctx context.Context, cmd []string) (string, error)
	DataDir() string
	GetGRPCConn() *grpc.ClientConn
}

// Config is a set of configurations required to create a Agent
//
// Constructed by (Builder).ToAgentConfig()
type Config struct {
	// NodeName is set for the consul agent name and container name
	// Equivalent to the -node command-line flag.
	// If empty, a random name will be generated
	NodeName string
	// NodeID is used to configure node_id in agent config file
	// Equivalent to the -node-id command-line flag.
	// If empty, a random name will be generated
	NodeID string

	// ExternalDataDir is data directory to copy consul data from, if set.
	// This directory contains subdirectories like raft, serf, services
	ExternalDataDir string

	ScratchDir    string
	CertVolume    string
	CACert        string
	JSON          string
	ConfigBuilder *ConfigBuilder
	Image         string
	Version       string
	Cmd           []string
	LogConsumer   testcontainers.LogConsumer

	// service defaults
	UseAPIWithTLS  bool // TODO
	UseGRPCWithTLS bool

	ACLEnabled bool
}

func (c *Config) DockerImage() string {
	return utils.DockerImage(c.Image, c.Version)
}

// Clone copies everything. It is the caller's job to replace fields that
// should be unique.
func (c Config) Clone() Config {
	c2 := c
	if c.Cmd != nil {
		copy(c2.Cmd, c.Cmd)
	}
	return c2
}

type decodeTarget struct {
	agentconfig.Config `mapstructure:",squash"`
}

// MutatebyAgentConfig mutates config by applying the fields in the input hclConfig
// Note that the precedence order is config > hclConfig, because user provider hclConfig
// may not work with the testing environment, e.g., data dir, agent name, etc.
// Currently only hcl config is allowed
func (c *Config) MutatebyAgentConfig(hclConfig string) error {
	rawConfigJson, err := convertHcl2Json(hclConfig)
	if err != nil {
		return fmt.Errorf("error converting to Json: %s", err)
	}

	// Merge 2 json
	mergedConfigJosn, err := jsonpatch.MergePatch([]byte(rawConfigJson), []byte(c.JSON))
	if err != nil {
		return fmt.Errorf("error merging configurations: %w", err)
	}

	c.JSON = string(mergedConfigJosn)
	return nil
}

// TODO: refactor away
type AgentInfo struct {
	CACertFile    string
	UseTLSForAPI  bool
	UseTLSForGRPC bool
	DebugURI      string
}

func convertHcl2Json(in string) (string, error) {
	var raw map[string]interface{}
	err := hcl.Decode(&raw, in)
	if err != nil {
		return "", err
	}

	var target decodeTarget
	var md mapstructure.Metadata
	d, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			// decode.HookWeakDecodeFromSlice is only necessary when reading from
			// an HCL config file. In the future we could omit it when reading from
			// JSON configs. It is left here for now to maintain backwards compat
			// for the unlikely scenario that someone is using malformed JSON configs
			// and expecting this behaviour to correct their config.
			decode.HookWeakDecodeFromSlice,
			decode.HookTranslateKeys,
		),
		Metadata: &md,
		Result:   &target,
	})
	if err != nil {
		return "", err
	}
	if err := d.Decode(raw); err != nil {
		return "", err
	}

	rawjson, err := json.MarshalIndent(target, "", "  ")
	if err != nil {
		return "", err
	}
	return string(rawjson), nil
}
