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

	buffer.WriteString(fmt.Sprintf("ID:           %s\n", role.ID))
	buffer.WriteString(fmt.Sprintf("Name:         %s\n", role.Name))
	if role.Partition != "" {
		buffer.WriteString(fmt.Sprintf("Partition:    %s\n", role.Partition))
	}
	if role.Namespace != "" {
		buffer.WriteString(fmt.Sprintf("Namespace:    %s\n", role.Namespace))
	}
	buffer.WriteString(fmt.Sprintf("Description:  %s\n", role.Description))
	if f.showMeta {
		buffer.WriteString(fmt.Sprintf("Hash:         %x\n", role.Hash))
		buffer.WriteString(fmt.Sprintf("Create Index: %d\n", role.CreateIndex))
		buffer.WriteString(fmt.Sprintf("Modify Index: %d\n", role.ModifyIndex))
	}
	if len(role.Policies) > 0 {
		buffer.WriteString(fmt.Sprintln("Policies:"))
		for _, policy := range role.Policies {
			buffer.WriteString(fmt.Sprintf("   %s - %s\n", policy.ID, policy.Name))
		}
	}
	if len(role.ServiceIdentities) > 0 {
		buffer.WriteString(fmt.Sprintln("Service Identities:"))
		for _, svcid := range role.ServiceIdentities {
			if len(svcid.Datacenters) > 0 {
				buffer.WriteString(fmt.Sprintf("   %s (Datacenters: %s)\n", svcid.ServiceName, strings.Join(svcid.Datacenters, ", ")))
			} else {
				buffer.WriteString(fmt.Sprintf("   %s (Datacenters: all)\n", svcid.ServiceName))
			}
		}
	}
	if len(role.NodeIdentities) > 0 {
		buffer.WriteString(fmt.Sprintln("Node Identities:"))
		for _, nodeid := range role.NodeIdentities {
			buffer.WriteString(fmt.Sprintf("   %s (Datacenter: %s)\n", nodeid.NodeName, nodeid.Datacenter))
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

	buffer.WriteString(fmt.Sprintf("%s:\n", role.Name))
	buffer.WriteString(fmt.Sprintf("   ID:           %s\n", role.ID))
	if role.Partition != "" {
		buffer.WriteString(fmt.Sprintf("   Partition:    %s\n", role.Partition))
	}
	if role.Namespace != "" {
		buffer.WriteString(fmt.Sprintf("   Namespace:    %s\n", role.Namespace))
	}
	buffer.WriteString(fmt.Sprintf("   Description:  %s\n", role.Description))
	if f.showMeta {
		buffer.WriteString(fmt.Sprintf("   Hash:         %x\n", role.Hash))
		buffer.WriteString(fmt.Sprintf("   Create Index: %d\n", role.CreateIndex))
		buffer.WriteString(fmt.Sprintf("   Modify Index: %d\n", role.ModifyIndex))
	}
	if len(role.Policies) > 0 {
		buffer.WriteString(fmt.Sprintln("   Policies:"))
		for _, policy := range role.Policies {
			buffer.WriteString(fmt.Sprintf("      %s - %s\n", policy.ID, policy.Name))
		}
	}
	if len(role.ServiceIdentities) > 0 {
		buffer.WriteString(fmt.Sprintln("   Service Identities:"))
		for _, svcid := range role.ServiceIdentities {
			if len(svcid.Datacenters) > 0 {
				buffer.WriteString(fmt.Sprintf("      %s (Datacenters: %s)\n", svcid.ServiceName, strings.Join(svcid.Datacenters, ", ")))
			} else {
				buffer.WriteString(fmt.Sprintf("      %s (Datacenters: all)\n", svcid.ServiceName))
			}
		}
	}

	if len(role.NodeIdentities) > 0 {
		buffer.WriteString(fmt.Sprintln("   Node Identities:"))
		for _, nodeid := range role.NodeIdentities {
			buffer.WriteString(fmt.Sprintf("      %s (Datacenter: %s)\n", nodeid.NodeName, nodeid.Datacenter))
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
