// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package config

type EnterpriseRuntimeConfig struct{}

func (c *RuntimeConfig) PartitionOrEmpty() string   { return "" }
func (c *RuntimeConfig) PartitionOrDefault() string { return "" }
