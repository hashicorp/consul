// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package token

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
)

const (
	PrettyFormat string = "pretty"
	JSONFormat   string = "json"

	WHITESPACE_2 string = "\t"
	WHITESPACE_4 string = "\t\t"
)

// Formatter defines methods provided by token command output formatter
type Formatter interface {
	FormatToken(token *api.ACLToken) (string, error)
	FormatTokenExpanded(token *api.ACLTokenExpanded) (string, error)
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

	fmt.Fprintf(&buffer, "Name:             %s\n", token.Name)
	fmt.Fprintf(&buffer, "AccessorID:       %s\n", token.AccessorID)
	fmt.Fprintf(&buffer, "SecretID:         %s\n", token.SecretID)

	if token.Partition != "" {
		fmt.Fprintf(&buffer, "Partition:        %s\n", token.Partition)
	}
	if token.Namespace != "" {
		fmt.Fprintf(&buffer, "Namespace:        %s\n", token.Namespace)
	}
	fmt.Fprintf(&buffer, "Description:      %s\n", token.Description)
	fmt.Fprintf(&buffer, "Local:            %t\n", token.Local)
	if token.AuthMethod != "" {
		fmt.Fprintf(&buffer, "Auth Method:      %s (Namespace: %s)\n", token.AuthMethod, token.AuthMethodNamespace)
	}
	fmt.Fprintf(&buffer, "Create Time:      %v\n", token.CreateTime)
	if token.ExpirationTime != nil && !token.ExpirationTime.IsZero() {
		fmt.Fprintf(&buffer, "Expiration Time:  %v\n", *token.ExpirationTime)
	}
	if f.showMeta {
		fmt.Fprintf(&buffer, "Hash:             %x\n", token.Hash)
		fmt.Fprintf(&buffer, "Create Index:     %d\n", token.CreateIndex)
		fmt.Fprintf(&buffer, "Modify Index:     %d\n", token.ModifyIndex)
	}
	if len(token.Policies) > 0 {
		fmt.Fprintln(&buffer, "Policies:")
		for _, policy := range token.Policies {
			fmt.Fprintf(&buffer, "   %s - %s\n", policy.ID, policy.Name)
		}
	}
	if len(token.Roles) > 0 {
		fmt.Fprintln(&buffer, "Roles:")
		for _, role := range token.Roles {
			fmt.Fprintf(&buffer, "   %s - %s\n", role.ID, role.Name)
		}
	}
	if len(token.ServiceIdentities) > 0 {
		fmt.Fprintln(&buffer, "Service Identities:")
		for _, svcid := range token.ServiceIdentities {
			if len(svcid.Datacenters) > 0 {
				fmt.Fprintf(&buffer, "   %s (Datacenters: %s)\n", svcid.ServiceName, strings.Join(svcid.Datacenters, ", "))
			} else {
				fmt.Fprintf(&buffer, "   %s (Datacenters: all)\n", svcid.ServiceName)
			}
		}
	}
	if len(token.NodeIdentities) > 0 {
		fmt.Fprintln(&buffer, "Node Identities:")
		for _, nodeid := range token.NodeIdentities {
			fmt.Fprintf(&buffer, "   %s (Datacenter: %s)\n", nodeid.NodeName, nodeid.Datacenter)
		}
	}
	if len(token.TemplatedPolicies) > 0 {
		fmt.Fprintln(&buffer, "Templated Policies:")
		for _, templatedPolicy := range token.TemplatedPolicies {
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

func (f *prettyFormatter) FormatTokenExpanded(token *api.ACLTokenExpanded) (string, error) {
	var buffer bytes.Buffer

	fmt.Fprintf(&buffer, "Name:             %s\n", token.Name)
	fmt.Fprintf(&buffer, "AccessorID:       %s\n", token.AccessorID)
	fmt.Fprintf(&buffer, "SecretID:         %s\n", token.SecretID)

	if token.Partition != "" {
		fmt.Fprintf(&buffer, "Partition:        %s\n", token.Partition)
	}
	if token.Namespace != "" {
		fmt.Fprintf(&buffer, "Namespace:        %s\n", token.Namespace)
	}
	fmt.Fprintf(&buffer, "Description:      %s\n", token.Description)
	fmt.Fprintf(&buffer, "Local:            %t\n", token.Local)
	if token.AuthMethod != "" {
		fmt.Fprintf(&buffer, "Auth Method:      %s (Namespace: %s)\n", token.AuthMethod, token.AuthMethodNamespace)
	}
	fmt.Fprintf(&buffer, "Create Time:      %v\n", token.CreateTime)
	if token.ExpirationTime != nil && !token.ExpirationTime.IsZero() {
		fmt.Fprintf(&buffer, "Expiration Time:  %v\n", *token.ExpirationTime)
	}
	if f.showMeta {
		fmt.Fprintf(&buffer, "Hash:             %x\n", token.Hash)
		fmt.Fprintf(&buffer, "Create Index:     %d\n", token.CreateIndex)
		fmt.Fprintf(&buffer, "Modify Index:     %d\n", token.ModifyIndex)
	}

	policies := make(map[string]api.ACLPolicy)
	roles := make(map[string]api.ACLRole)
	for _, policy := range token.ExpandedPolicies {
		policies[policy.ID] = policy
	}
	for _, role := range token.ExpandedRoles {
		roles[role.ID] = role
	}

	formatPolicy := func(policy api.ACLPolicy, indent string) {
		fmt.Fprintf(&buffer, indent+"Policy Name: %s\n", policy.Name)
		fmt.Fprintf(&buffer, indent+WHITESPACE_2+"ID: %s\n", policy.ID)
		fmt.Fprintf(&buffer, indent+WHITESPACE_2+"Description: %s\n", policy.Description)
		buffer.WriteString(indent + WHITESPACE_2 + "Rules:\n")
		buffer.WriteString(indent + WHITESPACE_4)
		buffer.WriteString(strings.ReplaceAll(policy.Rules, "\n", "\n"+indent+WHITESPACE_4))
		buffer.WriteString("\n\n")
	}

	if len(token.Policies) > 0 {
		buffer.WriteString("Policies:\n")
		for _, policyLink := range token.Policies {
			formatPolicy(policies[policyLink.ID], WHITESPACE_2)
		}
	}

	entMeta := acl.NewEnterpriseMetaWithPartition(token.Partition, token.Namespace)
	formatServiceIdentity := func(svcIdentity *api.ACLServiceIdentity, indent string) {
		if len(svcIdentity.Datacenters) > 0 {
			fmt.Fprintf(&buffer, indent+"Name: %s (Datacenters: %s)\n", svcIdentity.ServiceName, strings.Join(svcIdentity.Datacenters, ", "))
		} else {
			fmt.Fprintf(&buffer, indent+"Name: %s (Datacenters: all)\n", svcIdentity.ServiceName)
		}
		identity := structs.ACLServiceIdentity{ServiceName: svcIdentity.ServiceName, Datacenters: svcIdentity.Datacenters}
		policy := identity.SyntheticPolicy(&entMeta)
		displaySyntheticPolicy(policy, &buffer, indent)
	}
	if len(token.ServiceIdentities) > 0 {
		buffer.WriteString("Service Identities:\n")
		for _, svcIdentity := range token.ServiceIdentities {
			formatServiceIdentity(svcIdentity, WHITESPACE_2)
		}
	}

	formatNodeIdentity := func(nodeIdentity *api.ACLNodeIdentity, indent string) {
		fmt.Fprintf(&buffer, indent+"Name: %s (Datacenter: %s)\n", nodeIdentity.NodeName, nodeIdentity.Datacenter)
		identity := structs.ACLNodeIdentity{NodeName: nodeIdentity.NodeName, Datacenter: nodeIdentity.Datacenter}
		policy := identity.SyntheticPolicy(&entMeta)
		displaySyntheticPolicy(policy, &buffer, indent)
	}
	if len(token.NodeIdentities) > 0 {
		buffer.WriteString("Node Identities:\n")
		for _, nodeIdentity := range token.NodeIdentities {
			formatNodeIdentity(nodeIdentity, WHITESPACE_2)
		}
	}

	formatTemplatedPolicy := func(templatedPolicy *api.ACLTemplatedPolicy, indent string) {
		fmt.Fprintf(&buffer, indent+"%s\n", templatedPolicy.TemplateName)
		tp := structs.ACLTemplatedPolicy{
			TemplateName: templatedPolicy.TemplateName,
			Datacenters:  templatedPolicy.Datacenters,
		}
		if templatedPolicy.TemplateVariables != nil && templatedPolicy.TemplateVariables.Name != "" {
			tp.TemplateVariables = &structs.ACLTemplatedPolicyVariables{
				Name: templatedPolicy.TemplateVariables.Name,
			}
			fmt.Fprintf(&buffer, indent+WHITESPACE_2+"Name: %s\n", templatedPolicy.TemplateVariables.Name)
		}
		if len(templatedPolicy.Datacenters) > 0 {
			fmt.Fprintf(&buffer, indent+WHITESPACE_2+"Datacenters: %s\n", strings.Join(templatedPolicy.Datacenters, ", "))
		} else {
			buffer.WriteString(indent + WHITESPACE_2 + "Datacenters: all\n")
		}
		policy, _ := tp.SyntheticPolicy(&entMeta)
		displaySyntheticPolicy(policy, &buffer, indent)
	}
	if len(token.TemplatedPolicies) > 0 {
		buffer.WriteString("Templated Policies:\n")

		for _, templatedPolicy := range token.TemplatedPolicies {
			formatTemplatedPolicy(templatedPolicy, WHITESPACE_2)
		}
	}

	formatRole := func(role api.ACLRole, indent string) {
		fmt.Fprintf(&buffer, indent+"Role Name: %s\n", role.Name)
		fmt.Fprintf(&buffer, indent+WHITESPACE_2+"ID: %s\n", role.ID)
		fmt.Fprintf(&buffer, indent+WHITESPACE_2+"Description: %s\n", role.Description)

		if len(role.Policies) > 0 {
			buffer.WriteString(indent + WHITESPACE_2 + "Policies:\n")
			for _, policyLink := range role.Policies {
				formatPolicy(policies[policyLink.ID], indent+WHITESPACE_4)
			}
		}

		if len(role.ServiceIdentities) > 0 {
			buffer.WriteString(indent + WHITESPACE_2 + "Service Identities:\n")
			for _, svcIdentity := range role.ServiceIdentities {
				formatServiceIdentity(svcIdentity, indent+WHITESPACE_4)
			}
		}

		if len(role.NodeIdentities) > 0 {
			buffer.WriteString(indent + WHITESPACE_2 + "Node Identities:\n")
			for _, nodeIdentity := range role.NodeIdentities {
				formatNodeIdentity(nodeIdentity, indent+WHITESPACE_4)
			}
		}
	}
	if len(token.Roles) > 0 {
		buffer.WriteString("Roles:\n")
		for _, roleLink := range token.Roles {
			role := roles[roleLink.ID]
			formatRole(role, WHITESPACE_2)
		}
	}

	buffer.WriteString("=== End of Authorizer Layer 0: Token ===\n")

	if len(token.NamespaceDefaultPolicyIDs) > 0 || len(token.NamespaceDefaultRoleIDs) > 0 {
		buffer.WriteString("=== Start of Authorizer Layer 1: Token Namespace’s Defaults (Inherited) ===\n")
		fmt.Fprintf(&buffer, "Description: ACL Roles inherited by all Tokens in Namespace %q\n\n", token.Namespace)

		buffer.WriteString("Namespace Policy Defaults:\n")
		for _, policyID := range token.NamespaceDefaultPolicyIDs {
			formatPolicy(policies[policyID], WHITESPACE_2)
		}

		buffer.WriteString("Namespace Role Defaults:\n")
		for _, roleID := range token.NamespaceDefaultRoleIDs {
			formatRole(roles[roleID], WHITESPACE_2)
		}

		buffer.WriteString("=== End of Authorizer Layer 1: Token Namespace’s Defaults (Inherited) ===\n")
	}

	buffer.WriteString("=== Start of Authorizer Layer 2: Agent Configuration Defaults (Inherited) ===\n")
	buffer.WriteString("Description: Defined at request-time by the agent that resolves the ACL token; other agents may have different configuration defaults\n")
	fmt.Fprintf(&buffer, "Resolved By Agent: %q\n\n", token.ResolvedByAgent)
	fmt.Fprintf(&buffer, "Default Policy: %s\n", token.AgentACLDefaultPolicy)
	buffer.WriteString(WHITESPACE_2 + "Description: Backstop rule used if no preceding layer has a matching rule (refer to default_policy option in agent configuration)\n\n")
	fmt.Fprintf(&buffer, "Down Policy: %s\n", token.AgentACLDownPolicy)
	buffer.WriteString(WHITESPACE_2 + "Description: Defines what to do if this Token's information cannot be read from the primary_datacenter (refer to down_policy option in agent configuration)\n\n")

	return buffer.String(), nil
}

func displaySyntheticPolicy(policy *structs.ACLPolicy, buffer *bytes.Buffer, indent string) {
	fmt.Fprintf(buffer, indent+WHITESPACE_2+"Description: %s\n", policy.Description)
	buffer.WriteString(indent + WHITESPACE_2 + "Rules:")
	buffer.WriteString(strings.ReplaceAll(policy.Rules, "\n", "\n"+indent+WHITESPACE_4))
	buffer.WriteString("\n\n")
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
	fmt.Fprintf(&buffer, "Name:             %s\n", token.Name)
	fmt.Fprintf(&buffer, "AccessorID:       %s\n", token.AccessorID)
	fmt.Fprintf(&buffer, "SecretID:         %s\n", token.SecretID)

	if token.Partition != "" {
		fmt.Fprintf(&buffer, "Partition:        %s\n", token.Partition)
	}
	if token.Namespace != "" {
		fmt.Fprintf(&buffer, "Namespace:        %s\n", token.Namespace)
	}
	fmt.Fprintf(&buffer, "Description:      %s\n", token.Description)
	fmt.Fprintf(&buffer, "Local:            %t\n", token.Local)
	if token.AuthMethod != "" {
		fmt.Fprintf(&buffer, "Auth Method:      %s (Namespace: %s)\n", token.AuthMethod, token.AuthMethodNamespace)
	}
	fmt.Fprintf(&buffer, "Create Time:      %v\n", token.CreateTime)
	if token.ExpirationTime != nil && !token.ExpirationTime.IsZero() {
		fmt.Fprintf(&buffer, "Expiration Time:  %v\n", *token.ExpirationTime)
	}
	if f.showMeta {
		fmt.Fprintf(&buffer, "Hash:             %x\n", token.Hash)
		fmt.Fprintf(&buffer, "Create Index:     %d\n", token.CreateIndex)
		fmt.Fprintf(&buffer, "Modify Index:     %d\n", token.ModifyIndex)
	}
	if len(token.Policies) > 0 {
		fmt.Fprintln(&buffer, "Policies:")
		for _, policy := range token.Policies {
			fmt.Fprintf(&buffer, "   %s - %s\n", policy.ID, policy.Name)
		}
	}
	if len(token.Roles) > 0 {
		fmt.Fprintln(&buffer, "Roles:")
		for _, role := range token.Roles {
			fmt.Fprintf(&buffer, "   %s - %s\n", role.ID, role.Name)
		}
	}
	if len(token.ServiceIdentities) > 0 {
		fmt.Fprintln(&buffer, "Service Identities:")
		for _, svcid := range token.ServiceIdentities {
			if len(svcid.Datacenters) > 0 {
				fmt.Fprintf(&buffer, "   %s (Datacenters: %s)\n", svcid.ServiceName, strings.Join(svcid.Datacenters, ", "))
			} else {
				fmt.Fprintf(&buffer, "   %s (Datacenters: all)\n", svcid.ServiceName)
			}
		}
	}
	if len(token.NodeIdentities) > 0 {
		fmt.Fprintln(&buffer, "Node Identities:")
		for _, nodeid := range token.NodeIdentities {
			fmt.Fprintf(&buffer, "   %s (Datacenter: %s)\n", nodeid.NodeName, nodeid.Datacenter)
		}
	}

	if len(token.TemplatedPolicies) > 0 {
		fmt.Fprintln(&buffer, "Templated Policies:")
		for _, templatedPolicy := range token.TemplatedPolicies {
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

func (f *jsonFormatter) FormatTokenExpanded(token *api.ACLTokenExpanded) (string, error) {
	b, err := json.MarshalIndent(token, "", "    ")
	if err != nil {
		return "", fmt.Errorf("Failed to marshal token: %v", err)
	}
	return string(b), nil
}
