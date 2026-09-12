// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !linux

package iptables

import "errors"

// nftablesExecutor implements Provider and errors out on any non-linux OS.
type nftablesExecutor struct {
	cfg Config
}

func (n *nftablesExecutor) AddRule(_ string, _ ...string) {}

func (n *nftablesExecutor) ApplyRules(string) error {
	return errors.New("applying traffic redirection rules with 'nft' is not supported on this operating system; only linux OS is supported")
}

func (n *nftablesExecutor) Rules() []string {
	return nil
}

func (n *nftablesExecutor) ClearAllRules() {}
