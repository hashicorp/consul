// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/hashicorp/consul/envoyextensions/xdscommon"
)

type consulEnvoyVersions struct {
	ConsulVersion string
	EnvoyVersions []string
}

func main() {
	cev := consulEnvoyVersions{}

	// Get Consul Version
	data, err := os.ReadFile("./version/VERSION")
	if err != nil {
		panic(err)
	}
	cVersion := strings.TrimSpace(string(data))

	cev.EnvoyVersions = append(cev.EnvoyVersions, xdscommon.EnvoyVersions...)

	// ensure the versions are properly sorted latest to oldest
	sort.Sort(sort.Reverse(sort.StringSlice(cev.EnvoyVersions)))

	ceVersions := consulEnvoyVersions{
		ConsulVersion: cVersion,
		EnvoyVersions: cev.EnvoyVersions,
	}
	output, err := json.Marshal(ceVersions)
	if err != nil {
		panic(err)
	}
	fmt.Print(string(output))
}
