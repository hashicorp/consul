// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	// GRPCAddrEnvName defines an environment variable name which sets the gRPC
	// server address for the consul CLI.
	GRPCAddrEnvName = "CONSUL_GRPC_ADDR"

	// GRPCTLSEnvName defines an environment variable name which sets the gRPC
	// communication mode. Default is false in plaintext mode.
	GRPCTLSEnvName = "CONSUL_GRPC_TLS"

	// GRPCTLSVerifyEnvName defines an environment variable name which sets
	// whether to disable certificate checking.
	GRPCTLSVerifyEnvName = "CONSUL_GRPC_TLS_VERIFY"

	// GRPCClientCertEnvName defines an environment variable name which sets the
	// client cert file to use for talking to Consul over TLS.
	GRPCClientCertEnvName = "CONSUL_GRPC_CLIENT_CERT"

	// GRPCClientKeyEnvName defines an environment variable name which sets the
	// client key file to use for talking to Consul over TLS.
	GRPCClientKeyEnvName = "CONSUL_GRPC_CLIENT_KEY"

	// GRPCCAFileEnvName defines an environment variable name which sets the
	// CA file to use for talking to Consul gRPC over TLS.
	GRPCCAFileEnvName = "CONSUL_GRPC_CACERT"

	// GRPCCAPathEnvName defines an environment variable name which sets the
	// path to a directory of CA certs to use for talking to Consul gRPC over TLS.
	GRPCCAPathEnvName = "CONSUL_GRPC_CAPATH"
)

type GRPCConfig struct {
	// Address is the optional address of the Consul server in format of host:port.
	// It doesn't include schema
	Address string

	// GRPCTLS is the optional boolean flag to determine the communication protocol
	GRPCTLS bool

	// GRPCTLSVerify is the optional boolean flag to disable certificate checking.
	// Set to false only if you want to skip server verification
	GRPCTLSVerify bool

	// CertFile is the optional path to the certificate for Consul
	// communication. If this is set then you need to also set KeyFile.
	CertFile string

	// KeyFile is the optional path to the private key for Consul communication.
	// If this is set then you need to also set CertFile.
	KeyFile string

	// CAFile is the optional path to the CA certificate used for Consul
	// communication, defaults to the system bundle if not specified.
	CAFile string

	// CAPath is the optional path to a directory of CA certificates to use for
	// Consul communication, defaults to the system bundle if not specified.
	CAPath string
}

func GetDefaultGRPCConfig() *GRPCConfig {
	return &GRPCConfig{
		Address:       "127.0.0.1:8502",
		GRPCTLSVerify: false,
	}
}

func LoadGRPCConfig(defaultConfig *GRPCConfig) (*GRPCConfig, error) {
	if defaultConfig == nil {
		defaultConfig = GetDefaultGRPCConfig()
	}

	overwrittenConfig, err := loadEnvToDefaultConfig(defaultConfig)
	if err != nil {
		return nil, err
	}

	return overwrittenConfig, nil
}

func loadEnvToDefaultConfig(config *GRPCConfig) (*GRPCConfig, error) {
	if addr := os.Getenv(GRPCAddrEnvName); addr != "" {
		if strings.HasPrefix(strings.ToLower(addr), "https://") {
			config.GRPCTLS = true
		}
		config.Address = removeSchemaFromGRPCAddress(addr)
	}

	if tlsMode := os.Getenv(GRPCTLSEnvName); tlsMode != "" {
		doTLS, err := strconv.ParseBool(tlsMode)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", GRPCTLSEnvName, err)
		}
		if doTLS {
			config.GRPCTLS = true
		}
	}

	if v := os.Getenv(GRPCTLSVerifyEnvName); v != "" {
		doVerify, err := strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", GRPCTLSVerifyEnvName, err)
		}
		config.GRPCTLSVerify = doVerify
	}

	if v := os.Getenv(GRPCClientCertEnvName); v != "" {
		config.CertFile = v
	}

	if v := os.Getenv(GRPCClientKeyEnvName); v != "" {
		config.KeyFile = v
	}

	if caFile := os.Getenv(GRPCCAFileEnvName); caFile != "" {
		config.CAFile = caFile
	}

	if caPath := os.Getenv(GRPCCAPathEnvName); caPath != "" {
		config.CAPath = caPath
	}

	return config, nil
}
