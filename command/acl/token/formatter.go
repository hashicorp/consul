package token

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

// Formatter defines methods provided by token command output formatter
type Formatter interface {
	FormatToken(token *api.ACLToken) (string, error)
	FormatTokenList(tokens []*api.ACLTokenListEntry) (string, error)
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

func (f *prettyFormatter) FormatToken(token *api.ACLToken) (string, error) {
	var buffer bytes.Buffer

	buffer.WriteString(fmt.Sprintf("AccessorID:       %s\n", token.AccessorID))
	buffer.WriteString(fmt.Sprintf("SecretID:         %s\n", token.SecretID))
	if token.Namespace != "" {
		buffer.WriteString(fmt.Sprintf("Namespace:        %s\n", token.Namespace))
	}
	buffer.WriteString(fmt.Sprintf("Description:      %s\n", token.Description))
	buffer.WriteString(fmt.Sprintf("Local:            %t\n", token.Local))
	if token.AuthMethod != "" {
		buffer.WriteString(fmt.Sprintf("Auth Method:      %s\n", token.AuthMethod))
	}
	buffer.WriteString(fmt.Sprintf("Create Time:      %v\n", token.CreateTime))
	if token.ExpirationTime != nil && !token.ExpirationTime.IsZero() {
		buffer.WriteString(fmt.Sprintf("Expiration Time:  %v\n", *token.ExpirationTime))
	}
	if f.showMeta {
		buffer.WriteString(fmt.Sprintf("Hash:             %x\n", token.Hash))
		buffer.WriteString(fmt.Sprintf("Create Index:     %d\n", token.CreateIndex))
		buffer.WriteString(fmt.Sprintf("Modify Index:     %d\n", token.ModifyIndex))
	}
	if len(token.Policies) > 0 {
		buffer.WriteString(fmt.Sprintln("Policies:"))
		for _, policy := range token.Policies {
			buffer.WriteString(fmt.Sprintf("   %s - %s\n", policy.ID, policy.Name))
		}
	}
	if len(token.Roles) > 0 {
		buffer.WriteString(fmt.Sprintln("Roles:"))
		for _, role := range token.Roles {
			buffer.WriteString(fmt.Sprintf("   %s - %s\n", role.ID, role.Name))
		}
	}
	if len(token.ServiceIdentities) > 0 {
		buffer.WriteString(fmt.Sprintln("Service Identities:"))
		for _, svcid := range token.ServiceIdentities {
			if len(svcid.Datacenters) > 0 {
				buffer.WriteString(fmt.Sprintf("   %s (Datacenters: %s)\n", svcid.ServiceName, strings.Join(svcid.Datacenters, ", ")))
			} else {
				buffer.WriteString(fmt.Sprintf("   %s (Datacenters: all)\n", svcid.ServiceName))
			}
		}
	}
	if len(token.NodeIdentities) > 0 {
		buffer.WriteString(fmt.Sprintln("Node Identities:"))
		for _, nodeid := range token.NodeIdentities {
			buffer.WriteString(fmt.Sprintf("   %s (Datacenter: %s)\n", nodeid.NodeName, nodeid.Datacenter))
		}
	}
	if token.Rules != "" {
		buffer.WriteString(fmt.Sprintln("Rules:"))
		buffer.WriteString(fmt.Sprintln(token.Rules))
	}

	return buffer.String(), nil
}

func (f *prettyFormatter) FormatTokenList(tokens []*api.ACLTokenListEntry) (string, error) {
	var buffer bytes.Buffer

	first := true
	for _, token := range tokens {
		if first {
			first = false
		} else {
			buffer.WriteString("\n")
		}
		buffer.WriteString(f.formatTokenListEntry(token))
	}

	return buffer.String(), nil
}

func (f *prettyFormatter) formatTokenListEntry(token *api.ACLTokenListEntry) string {
	var buffer bytes.Buffer

	buffer.WriteString(fmt.Sprintf("AccessorID:       %s\n", token.AccessorID))
	if token.Namespace != "" {
		buffer.WriteString(fmt.Sprintf("Namespace:        %s\n", token.Namespace))
	}
	buffer.WriteString(fmt.Sprintf("Description:      %s\n", token.Description))
	buffer.WriteString(fmt.Sprintf("Local:            %t\n", token.Local))
	if token.AuthMethod != "" {
		buffer.WriteString(fmt.Sprintf("Auth Method:      %s\n", token.AuthMethod))
	}
	buffer.WriteString(fmt.Sprintf("Create Time:      %v\n", token.CreateTime))
	if token.ExpirationTime != nil && !token.ExpirationTime.IsZero() {
		buffer.WriteString(fmt.Sprintf("Expiration Time:  %v\n", *token.ExpirationTime))
	}
	buffer.WriteString(fmt.Sprintf("Legacy:           %t\n", token.Legacy))
	if f.showMeta {
		buffer.WriteString(fmt.Sprintf("Hash:             %x\n", token.Hash))
		buffer.WriteString(fmt.Sprintf("Create Index:     %d\n", token.CreateIndex))
		buffer.WriteString(fmt.Sprintf("Modify Index:     %d\n", token.ModifyIndex))
	}
	if len(token.Policies) > 0 {
		buffer.WriteString(fmt.Sprintln("Policies:"))
		for _, policy := range token.Policies {
			buffer.WriteString(fmt.Sprintf("   %s - %s\n", policy.ID, policy.Name))
		}
	}
	if len(token.Roles) > 0 {
		buffer.WriteString(fmt.Sprintln("Roles:"))
		for _, role := range token.Roles {
			buffer.WriteString(fmt.Sprintf("   %s - %s\n", role.ID, role.Name))
		}
	}
	if len(token.ServiceIdentities) > 0 {
		buffer.WriteString(fmt.Sprintln("Service Identities:"))
		for _, svcid := range token.ServiceIdentities {
			if len(svcid.Datacenters) > 0 {
				buffer.WriteString(fmt.Sprintf("   %s (Datacenters: %s)\n", svcid.ServiceName, strings.Join(svcid.Datacenters, ", ")))
			} else {
				buffer.WriteString(fmt.Sprintf("   %s (Datacenters: all)\n", svcid.ServiceName))
			}
		}
	}
	if len(token.NodeIdentities) > 0 {
		buffer.WriteString(fmt.Sprintln("Service Identities:"))
		for _, svcid := range token.ServiceIdentities {
			if len(svcid.Datacenters) > 0 {
				buffer.WriteString(fmt.Sprintf("   %s (Datacenters: %s)\n", svcid.ServiceName, strings.Join(svcid.Datacenters, ", ")))
			} else {
				buffer.WriteString(fmt.Sprintf("   %s (Datacenters: all)\n", svcid.ServiceName))
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

func (f *jsonFormatter) FormatTokenList(tokens []*api.ACLTokenListEntry) (string, error) {
	b, err := json.MarshalIndent(tokens, "", "    ")
	if err != nil {
		return "", fmt.Errorf("Failed to marshal tokens: %v", err)
	}
	return string(b), nil
}

func (f *jsonFormatter) FormatToken(token *api.ACLToken) (string, error) {
	b, err := json.MarshalIndent(token, "", "    ")
	if err != nil {
		return "", fmt.Errorf("Failed to marshal token: %v", err)
	}
	return string(b), nil
}
