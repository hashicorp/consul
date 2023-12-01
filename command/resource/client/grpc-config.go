// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"os"
)

const (
	// GRPCAddrEnvName defines an environment variable name which sets the gRPC
	// server address for the consul CLI.
	GRPCAddrEnvName = "CONSUL_GRPC_ADDR"
)

type GRPCConfig struct {
	Address string
}

func GetDefaultGRPCConfig() *GRPCConfig {
	return &GRPCConfig{
		Address: "localhost:8502",
	}
}

func LoadGRPCConfig(defaultConfig *GRPCConfig) *GRPCConfig {
	if defaultConfig == nil {
		defaultConfig = GetDefaultGRPCConfig()
	}

	overwrittenConfig := loadEnvToDefaultConfig(defaultConfig)

	return overwrittenConfig
}

func loadEnvToDefaultConfig(config *GRPCConfig) *GRPCConfig {

	if addr := os.Getenv(GRPCAddrEnvName); addr != "" {
		config.Address = addr
	}

	return config
}
