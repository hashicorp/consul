// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package acl

type enterprisePolicyRulesMergeContext struct{}

func (ctx *enterprisePolicyRulesMergeContext) init() {
	// do nothing
}

func (ctx *enterprisePolicyRulesMergeContext) merge(*EnterprisePolicyRules) {
	// do nothing
}

func (ctx *enterprisePolicyRulesMergeContext) fill(*EnterprisePolicyRules) {
	// do nothing
}
