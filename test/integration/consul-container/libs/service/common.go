// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package service

import (
	_ "embed"
	"os"
)

const (
	envoyEnvKey   = "ENVOY_VERSION"
	envoyLogLevel = "debug"
	envoyVersion  = "1.23.1"

	hashicorpDockerProxy = "docker.mirror.hashicorp.services"
)

func getEnvoyVersion() string {
	if version, ok := os.LookupEnv(envoyEnvKey); ok && version != "" {
		return version
	}
	return envoyVersion
}
