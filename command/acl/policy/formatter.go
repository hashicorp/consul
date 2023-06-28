// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

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

	buffer.WriteString(fmt.Sprintf("ID:           %s\n", policy.ID))
	buffer.WriteString(fmt.Sprintf("Name:         %s\n", policy.Name))
	if policy.Partition != "" {
		buffer.WriteString(fmt.Sprintf("Partition:    %s\n", policy.Partition))
	}
	if policy.Namespace != "" {
		buffer.WriteString(fmt.Sprintf("Namespace:    %s\n", policy.Namespace))
	}
	buffer.WriteString(fmt.Sprintf("Description:  %s\n", policy.Description))
	buffer.WriteString(fmt.Sprintf("Datacenters:  %s\n", strings.Join(policy.Datacenters, ", ")))
	if f.showMeta {
		buffer.WriteString(fmt.Sprintf("Hash:         %x\n", policy.Hash))
		buffer.WriteString(fmt.Sprintf("Create Index: %d\n", policy.CreateIndex))
		buffer.WriteString(fmt.Sprintf("Modify Index: %d\n", policy.ModifyIndex))
	}
	buffer.WriteString(fmt.Sprintln("Rules:"))
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

	buffer.WriteString(fmt.Sprintf("%s:\n", policy.Name))
	buffer.WriteString(fmt.Sprintf("   ID:           %s\n", policy.ID))
	if policy.Partition != "" {
		buffer.WriteString(fmt.Sprintf("   Partition:    %s\n", policy.Partition))
	}
	if policy.Namespace != "" {
		buffer.WriteString(fmt.Sprintf("   Namespace:    %s\n", policy.Namespace))
	}
	buffer.WriteString(fmt.Sprintf("   Description:  %s\n", policy.Description))
	buffer.WriteString(fmt.Sprintf("   Datacenters:  %s\n", strings.Join(policy.Datacenters, ", ")))
	if f.showMeta {
		buffer.WriteString(fmt.Sprintf("   Hash:         %x\n", policy.Hash))
		buffer.WriteString(fmt.Sprintf("   Create Index: %d\n", policy.CreateIndex))
		buffer.WriteString(fmt.Sprintf("   Modify Index: %d\n", policy.ModifyIndex))
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
