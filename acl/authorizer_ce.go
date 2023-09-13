// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package acl

// AuthorizerContext contains extra information that can be
// used in the determination of an ACL enforcement decision.
type AuthorizerContext struct {
	// Peer is the name of the peer that the resource was imported from.
	Peer string
}

func (c *AuthorizerContext) PeerOrEmpty() string {
	if c == nil {
		return ""
	}
	return c.Peer
}

// enterpriseAuthorizer stub interface
type enterpriseAuthorizer interface{}

func enforceEnterprise(_ Authorizer, _ Resource, _ string, _ string, _ *AuthorizerContext) (bool, EnforcementDecision, error) {
	return false, Deny, nil
}
