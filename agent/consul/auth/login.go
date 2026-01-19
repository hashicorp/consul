// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

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
		TemplatedPolicies: bindings.TemplatedPolicies,
		Roles:             bindings.Roles,
		Policies:          bindings.Policies,
		EnterpriseMeta:    bindings.EnterpriseMeta,
	}
	token.FillWithEnterpriseMeta(&authMethod.EnterpriseMeta)

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

// TokenForVerifiedIdentityWithMeta creates an ACLToken for the given identity verified
// by an auth method, using the auth method's TokenNameFormat if specified.
func (l *Login) TokenForVerifiedIdentityWithMeta(identity *authmethod.Identity, authMethod *structs.ACLAuthMethod, meta map[string]string) (*structs.ACLToken, error) {
	// Format the token description using the auth method's TokenNameFormat
	description, err := FormatTokenName(authMethod.TokenNameFormat, authMethod.Name, identity, meta)
	if err != nil {
		return nil, fmt.Errorf("failed to format token name: %w", err)
	}

	return l.TokenForVerifiedIdentity(identity, authMethod, description)
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

// FormatTokenName formats the token name using the provided template and identity.
// The template can use variables like ${auth_method}, ${identity.username}, ${identity.email}, etc.
// If the template is empty, it returns the default description.
func FormatTokenName(tokenNameFormat string, authMethodName string, identity *authmethod.Identity, meta map[string]string) (string, error) {
	if tokenNameFormat == "" {
		return BuildTokenDescription("token created via login", meta)
	}

	// Build template data from identity and auth method
	data := make(map[string]interface{})
	data["auth_method"] = authMethodName

	// Add identity fields
	if identity != nil {
		identityFields := make(map[string]interface{})
		for k, v := range identity.ProjectedVars {
			identityFields[k] = v
		}
		data["identity"] = identityFields
	}

	// Add meta fields
	if len(meta) > 0 {
		data["meta"] = meta
	}

	// Convert ${var} syntax to {{.var}} for Go templates
	// Support patterns like ${auth_method}, ${identity.username}, ${meta.key}
	templateStr := tokenNameFormat

	// Replace ${auth_method} with {{.auth_method}}
	templateStr = strings.ReplaceAll(templateStr, "${auth_method}", "{{.auth_method}}")

	// Replace ${identity.xxx} with {{.identity.xxx}}
	// Use a more robust replacement that handles any field name
	for strings.Contains(templateStr, "${identity.") {
		start := strings.Index(templateStr, "${identity.")
		if start == -1 {
			break
		}
		end := strings.Index(templateStr[start:], "}")
		if end == -1 {
			break
		}
		end += start

		placeholder := templateStr[start : end+1]
		fieldName := placeholder[2 : len(placeholder)-1] // Remove ${ and }
		replacement := "{{." + fieldName + "}}"
		templateStr = strings.ReplaceAll(templateStr, placeholder, replacement)
	}

	// Replace ${meta.xxx} with {{.meta.xxx}}
	for strings.Contains(templateStr, "${meta.") {
		start := strings.Index(templateStr, "${meta.")
		if start == -1 {
			break
		}
		end := strings.Index(templateStr[start:], "}")
		if end == -1 {
			break
		}
		end += start

		placeholder := templateStr[start : end+1]
		fieldName := placeholder[2 : len(placeholder)-1] // Remove ${ and }
		replacement := "{{." + fieldName + "}}"
		templateStr = strings.ReplaceAll(templateStr, placeholder, replacement)
	}

	// Parse and execute template
	tmpl, err := template.New("token-name").Option("missingkey=zero").Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse token name format template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute token name format template: %w", err)
	}

	return buf.String(), nil
}
