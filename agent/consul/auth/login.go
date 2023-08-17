// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package auth

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/authmethod"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
)

// Login wraps the process of creating an ACLToken from the identity verified
// by an auth method.
type Login struct {
	binder *Binder
	writer *TokenWriter
}

// NewLogin returns a new Login with the given binder and writer.
func NewLogin(binder *Binder, writer *TokenWriter) *Login {
	return &Login{binder, writer}
}

// TokenForVerifiedIdentity creates an ACLToken for the given identity verified
// by an auth method.
func (l *Login) TokenForVerifiedIdentity(identity *authmethod.Identity, authMethod *structs.ACLAuthMethod, description string) (*structs.ACLToken, error) {
	bindings, err := l.binder.Bind(authMethod, identity)
	switch {
	case err != nil:
		return nil, err
	case bindings.None():
		// We try to prevent the creation of a useless token without taking a trip
		// through Raft and the state store if we can.
		return nil, acl.ErrPermissionDenied
	}

	token := &structs.ACLToken{
		Description:       description,
		Local:             authMethod.TokenLocality != "global", // TokenWriter prevents the creation of global tokens in secondary datacenters.
		AuthMethod:        authMethod.Name,
		ExpirationTTL:     authMethod.MaxTokenTTL,
		ServiceIdentities: bindings.ServiceIdentities,
		NodeIdentities:    bindings.NodeIdentities,
		Roles:             bindings.Roles,
		EnterpriseMeta:    bindings.EnterpriseMeta,
	}
	token.ACLAuthMethodEnterpriseMeta.FillWithEnterpriseMeta(&authMethod.EnterpriseMeta)

	updated, err := l.writer.Create(token, true)
	switch {
	case err != nil && strings.Contains(err.Error(), state.ErrTokenHasNoPrivileges.Error()):
		// If we were in a slight race with a role delete operation then we may
		// still end up failing to insert an unprivileged token in the state
		// machine instead. Return the same error as earlier so it doesn't
		// actually matter which one prevents the insertion.
		return nil, acl.ErrPermissionDenied
	case err != nil:
		return nil, err
	}
	return updated, nil
}

// BuildTokenDescription builds a description for an ACLToken by encoding the
// given meta as JSON and applying the prefix.
func BuildTokenDescription(prefix string, meta map[string]string) (string, error) {
	if len(meta) == 0 {
		return prefix, nil
	}

	d, err := json.Marshal(meta)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s: %s", prefix, d), nil
}
