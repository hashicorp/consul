// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !linux
// +build !linux

package iptables

import "errors"

// iptablesExecutor implements IptablesProvider and errors out on any non-linux OS.
type iptablesExecutor struct {
	cfg Config
}

func (i *iptablesExecutor) AddRule(_ string, _ ...string) {}

func (i *iptablesExecutor) ApplyRules() error {
	return errors.New("applying traffic redirection rules with 'iptables' is not supported on this operating system; only linux OS is supported")
}

func (i *iptablesExecutor) Rules() []string {
	return nil
}
