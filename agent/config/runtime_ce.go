// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package config

type EnterpriseRuntimeConfig struct{}

func (c *RuntimeConfig) PartitionOrEmpty() string   { return "" }
func (c *RuntimeConfig) PartitionOrDefault() string { return "" }
