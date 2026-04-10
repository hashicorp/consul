// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package policy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/consul/api"
)

const (
	PrettyFormat string = "pretty"
	JSONFormat   string = "json"
)

// Formatter defines methods provided by policy command output formatter
type Formatter interface {
	FormatPolicy(policy *api.ACLPolicy) (string, error)
	FormatPolicyList(policies []*api.ACLPolicyListEntry) (string, error)
}

// GetSupportedFormats returns supported formats
func GetSupportedFormats() []string {
	return []string{PrettyFormat, JSONFormat}
}

// NewFormatter returns Formatter implementation
func NewFormatter(format string, showMeta bool) (formatter Formatter, err error) {
	switch format {
	case PrettyFormat:
		formatter = newPrettyFormatter(showMeta)
	case JSONFormat:
		formatter = newJSONFormatter(showMeta)
	default:
		err = fmt.Errorf("Unknown format: %s", format)
	}

	return formatter, err
}

func newPrettyFormatter(showMeta bool) Formatter {
	return &prettyFormatter{showMeta}
}

type prettyFormatter struct {
	showMeta bool
}

func (f *prettyFormatter) FormatPolicy(policy *api.ACLPolicy) (string, error) {
	var buffer bytes.Buffer

	fmt.Fprintf(&buffer, "ID:           %s\n", policy.ID)
	fmt.Fprintf(&buffer, "Name:         %s\n", policy.Name)
	if policy.Partition != "" {
		fmt.Fprintf(&buffer, "Partition:    %s\n", policy.Partition)
	}
	if policy.Namespace != "" {
		fmt.Fprintf(&buffer, "Namespace:    %s\n", policy.Namespace)
	}
	fmt.Fprintf(&buffer, "Description:  %s\n", policy.Description)
	fmt.Fprintf(&buffer, "Datacenters:  %s\n", strings.Join(policy.Datacenters, ", "))
	if f.showMeta {
		fmt.Fprintf(&buffer, "Hash:         %x\n", policy.Hash)
		fmt.Fprintf(&buffer, "Create Index: %d\n", policy.CreateIndex)
		fmt.Fprintf(&buffer, "Modify Index: %d\n", policy.ModifyIndex)
	}
	fmt.Fprintln(&buffer, "Rules:")
	buffer.WriteString(policy.Rules)

	return buffer.String(), nil
}

func (f *prettyFormatter) FormatPolicyList(policies []*api.ACLPolicyListEntry) (string, error) {
	var buffer bytes.Buffer

	for _, policy := range policies {
		buffer.WriteString(f.formatPolicyListEntry(policy))
	}

	return buffer.String(), nil
}

func (f *prettyFormatter) formatPolicyListEntry(policy *api.ACLPolicyListEntry) string {
	var buffer bytes.Buffer

	fmt.Fprintf(&buffer, "%s:\n", policy.Name)
	fmt.Fprintf(&buffer, "   ID:           %s\n", policy.ID)
	if policy.Partition != "" {
		fmt.Fprintf(&buffer, "   Partition:    %s\n", policy.Partition)
	}
	if policy.Namespace != "" {
		fmt.Fprintf(&buffer, "   Namespace:    %s\n", policy.Namespace)
	}
	fmt.Fprintf(&buffer, "   Description:  %s\n", policy.Description)
	fmt.Fprintf(&buffer, "   Datacenters:  %s\n", strings.Join(policy.Datacenters, ", "))
	if f.showMeta {
		fmt.Fprintf(&buffer, "   Hash:         %x\n", policy.Hash)
		fmt.Fprintf(&buffer, "   Create Index: %d\n", policy.CreateIndex)
		fmt.Fprintf(&buffer, "   Modify Index: %d\n", policy.ModifyIndex)
	}

	return buffer.String()
}

func newJSONFormatter(showMeta bool) Formatter {
	return &jsonFormatter{showMeta}
}

type jsonFormatter struct {
	showMeta bool
}

func (f *jsonFormatter) FormatPolicy(policy *api.ACLPolicy) (string, error) {
	b, err := json.MarshalIndent(policy, "", "    ")
	if err != nil {
		return "", fmt.Errorf("Failed to marshal policy: %v", err)
	}
	return string(b), nil
}

func (f *jsonFormatter) FormatPolicyList(policies []*api.ACLPolicyListEntry) (string, error) {
	b, err := json.MarshalIndent(policies, "", "    ")
	if err != nil {
		return "", fmt.Errorf("Failed to marshal policies: %v", err)
	}
	return string(b), nil
}
