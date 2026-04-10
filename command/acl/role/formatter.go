// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package role

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

// Formatter defines methods provided by role command output formatter
type Formatter interface {
	FormatRole(role *api.ACLRole) (string, error)
	FormatRoleList(roles []*api.ACLRole) (string, error)
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

func (f *prettyFormatter) FormatRole(role *api.ACLRole) (string, error) {
	var buffer bytes.Buffer

	fmt.Fprintf(&buffer, "ID:           %s\n", role.ID)
	fmt.Fprintf(&buffer, "Name:         %s\n", role.Name)
	if role.Partition != "" {
		fmt.Fprintf(&buffer, "Partition:    %s\n", role.Partition)
	}
	if role.Namespace != "" {
		fmt.Fprintf(&buffer, "Namespace:    %s\n", role.Namespace)
	}
	fmt.Fprintf(&buffer, "Description:  %s\n", role.Description)
	if f.showMeta {
		fmt.Fprintf(&buffer, "Hash:         %x\n", role.Hash)
		fmt.Fprintf(&buffer, "Create Index: %d\n", role.CreateIndex)
		fmt.Fprintf(&buffer, "Modify Index: %d\n", role.ModifyIndex)
	}
	if len(role.Policies) > 0 {
		fmt.Fprintln(&buffer, "Policies:")
		for _, policy := range role.Policies {
			fmt.Fprintf(&buffer, "   %s - %s\n", policy.ID, policy.Name)
		}
	}
	if len(role.ServiceIdentities) > 0 {
		fmt.Fprintln(&buffer, "Service Identities:")
		for _, svcid := range role.ServiceIdentities {
			if len(svcid.Datacenters) > 0 {
				fmt.Fprintf(&buffer, "   %s (Datacenters: %s)\n", svcid.ServiceName, strings.Join(svcid.Datacenters, ", "))
			} else {
				fmt.Fprintf(&buffer, "   %s (Datacenters: all)\n", svcid.ServiceName)
			}
		}
	}
	if len(role.NodeIdentities) > 0 {
		fmt.Fprintln(&buffer, "Node Identities:")
		for _, nodeid := range role.NodeIdentities {
			fmt.Fprintf(&buffer, "   %s (Datacenter: %s)\n", nodeid.NodeName, nodeid.Datacenter)
		}
	}
	if len(role.TemplatedPolicies) > 0 {
		fmt.Fprintln(&buffer, "Templated Policies:")
		for _, templatedPolicy := range role.TemplatedPolicies {
			fmt.Fprintf(&buffer, "   %s\n", templatedPolicy.TemplateName)
			if templatedPolicy.TemplateVariables != nil && templatedPolicy.TemplateVariables.Name != "" {
				fmt.Fprintf(&buffer, "      Name: %s\n", templatedPolicy.TemplateVariables.Name)
			}
			if len(templatedPolicy.Datacenters) > 0 {
				fmt.Fprintf(&buffer, "      Datacenters: %s\n", strings.Join(templatedPolicy.Datacenters, ", "))
			} else {
				buffer.WriteString("      Datacenters: all\n")
			}
		}
	}

	return buffer.String(), nil
}

func (f *prettyFormatter) FormatRoleList(roles []*api.ACLRole) (string, error) {
	var buffer bytes.Buffer

	for _, role := range roles {
		buffer.WriteString(f.formatRoleListEntry(role))
	}

	return buffer.String(), nil
}

func (f *prettyFormatter) formatRoleListEntry(role *api.ACLRole) string {
	var buffer bytes.Buffer

	fmt.Fprintf(&buffer, "%s:\n", role.Name)
	fmt.Fprintf(&buffer, "   ID:           %s\n", role.ID)
	if role.Partition != "" {
		fmt.Fprintf(&buffer, "   Partition:    %s\n", role.Partition)
	}
	if role.Namespace != "" {
		fmt.Fprintf(&buffer, "   Namespace:    %s\n", role.Namespace)
	}
	fmt.Fprintf(&buffer, "   Description:  %s\n", role.Description)
	if f.showMeta {
		fmt.Fprintf(&buffer, "   Hash:         %x\n", role.Hash)
		fmt.Fprintf(&buffer, "   Create Index: %d\n", role.CreateIndex)
		fmt.Fprintf(&buffer, "   Modify Index: %d\n", role.ModifyIndex)
	}
	if len(role.Policies) > 0 {
		fmt.Fprintln(&buffer, "   Policies:")
		for _, policy := range role.Policies {
			fmt.Fprintf(&buffer, "      %s - %s\n", policy.ID, policy.Name)
		}
	}
	if len(role.ServiceIdentities) > 0 {
		fmt.Fprintln(&buffer, "   Service Identities:")
		for _, svcid := range role.ServiceIdentities {
			if len(svcid.Datacenters) > 0 {
				fmt.Fprintf(&buffer, "      %s (Datacenters: %s)\n", svcid.ServiceName, strings.Join(svcid.Datacenters, ", "))
			} else {
				fmt.Fprintf(&buffer, "      %s (Datacenters: all)\n", svcid.ServiceName)
			}
		}
	}

	if len(role.NodeIdentities) > 0 {
		fmt.Fprintln(&buffer, "   Node Identities:")
		for _, nodeid := range role.NodeIdentities {
			fmt.Fprintf(&buffer, "      %s (Datacenter: %s)\n", nodeid.NodeName, nodeid.Datacenter)
		}
	}

	if len(role.TemplatedPolicies) > 0 {
		fmt.Fprintln(&buffer, "   Templated Policies:")
		for _, templatedPolicy := range role.TemplatedPolicies {
			fmt.Fprintf(&buffer, "      %s\n", templatedPolicy.TemplateName)
			if templatedPolicy.TemplateVariables != nil && templatedPolicy.TemplateVariables.Name != "" {
				fmt.Fprintf(&buffer, "         Name: %s\n", templatedPolicy.TemplateVariables.Name)
			}
			if len(templatedPolicy.Datacenters) > 0 {
				fmt.Fprintf(&buffer, "         Datacenters: %s\n", strings.Join(templatedPolicy.Datacenters, ", "))
			} else {
				buffer.WriteString("         Datacenters: all\n")
			}
		}
	}

	return buffer.String()
}

func newJSONFormatter(showMeta bool) Formatter {
	return &jsonFormatter{showMeta}
}

type jsonFormatter struct {
	showMeta bool
}

func (f *jsonFormatter) FormatRole(role *api.ACLRole) (string, error) {
	b, err := json.MarshalIndent(role, "", "    ")
	if err != nil {
		return "", fmt.Errorf("Failed to marshal role: %v", err)
	}
	return string(b), nil
}

func (f *jsonFormatter) FormatRoleList(roles []*api.ACLRole) (string, error) {
	b, err := json.MarshalIndent(roles, "", "    ")
	if err != nil {
		return "", fmt.Errorf("Failed to marshal roles: %v", err)
	}
	return string(b), nil
}
