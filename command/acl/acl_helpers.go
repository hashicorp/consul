package acl

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/mitchellh/cli"
)

func PrintToken(token *api.ACLToken, ui cli.Ui, showMeta bool) {
	ui.Info(fmt.Sprintf("AccessorID:       %s", token.AccessorID))
	ui.Info(fmt.Sprintf("SecretID:         %s", token.SecretID))
	ui.Info(fmt.Sprintf("Description:      %s", token.Description))
	ui.Info(fmt.Sprintf("Local:            %t", token.Local))
	ui.Info(fmt.Sprintf("Create Time:      %v", token.CreateTime))
	if !token.ExpirationTime.IsZero() {
		ui.Info(fmt.Sprintf("Expiration Time:  %v", token.ExpirationTime))
	}
	if showMeta {
		ui.Info(fmt.Sprintf("Hash:             %x", token.Hash))
		ui.Info(fmt.Sprintf("Create Index:     %d", token.CreateIndex))
		ui.Info(fmt.Sprintf("Modify Index:     %d", token.ModifyIndex))
	}
	ui.Info(fmt.Sprintf("Policies:"))
	for _, policy := range token.Policies {
		ui.Info(fmt.Sprintf("   %s - %s", policy.ID, policy.Name))
	}
	if token.Rules != "" {
		ui.Info(fmt.Sprintf("Rules:"))
		ui.Info(token.Rules)
	}
}

func PrintTokenListEntry(token *api.ACLTokenListEntry, ui cli.Ui, showMeta bool) {
	ui.Info(fmt.Sprintf("AccessorID:       %s", token.AccessorID))
	ui.Info(fmt.Sprintf("Description:      %s", token.Description))
	ui.Info(fmt.Sprintf("Local:            %t", token.Local))
	ui.Info(fmt.Sprintf("Create Time:      %v", token.CreateTime))
	if !token.ExpirationTime.IsZero() {
		ui.Info(fmt.Sprintf("Expiration Time:  %v", token.ExpirationTime))
	}
	ui.Info(fmt.Sprintf("Legacy:           %t", token.Legacy))
	if showMeta {
		ui.Info(fmt.Sprintf("Hash:             %x", token.Hash))
		ui.Info(fmt.Sprintf("Create Index:     %d", token.CreateIndex))
		ui.Info(fmt.Sprintf("Modify Index:     %d", token.ModifyIndex))
	}
	ui.Info(fmt.Sprintf("Policies:"))
	for _, policy := range token.Policies {
		ui.Info(fmt.Sprintf("   %s - %s", policy.ID, policy.Name))
	}
}

func PrintPolicy(policy *api.ACLPolicy, ui cli.Ui, showMeta bool) {
	ui.Info(fmt.Sprintf("ID:           %s", policy.ID))
	ui.Info(fmt.Sprintf("Name:         %s", policy.Name))
	ui.Info(fmt.Sprintf("Description:  %s", policy.Description))
	ui.Info(fmt.Sprintf("Datacenters:  %s", strings.Join(policy.Datacenters, ", ")))
	if showMeta {
		ui.Info(fmt.Sprintf("Hash:         %x", policy.Hash))
		ui.Info(fmt.Sprintf("Create Index: %d", policy.CreateIndex))
		ui.Info(fmt.Sprintf("Modify Index: %d", policy.ModifyIndex))
	}
	ui.Info(fmt.Sprintf("Rules:"))
	ui.Info(policy.Rules)
}

func PrintPolicyListEntry(policy *api.ACLPolicyListEntry, ui cli.Ui, showMeta bool) {
	ui.Info(fmt.Sprintf("%s:", policy.Name))
	ui.Info(fmt.Sprintf("   ID:           %s", policy.ID))
	ui.Info(fmt.Sprintf("   Description:  %s", policy.Description))
	ui.Info(fmt.Sprintf("   Datacenters:  %s", strings.Join(policy.Datacenters, ", ")))
	if showMeta {
		ui.Info(fmt.Sprintf("   Hash:         %x", policy.Hash))
		ui.Info(fmt.Sprintf("   Create Index: %d", policy.CreateIndex))
		ui.Info(fmt.Sprintf("   Modify Index: %d", policy.ModifyIndex))
	}
}

func GetTokenIDFromPartial(client *api.Client, partialID string) (string, error) {
	if partialID == "anonymous" {
		return structs.ACLTokenAnonymousID, nil
	}

	// the full UUID string was given
	if len(partialID) == 36 {
		return partialID, nil
	}

	tokens, _, err := client.ACL().TokenList(nil)
	if err != nil {
		return "", err
	}

	tokenID := ""
	for _, token := range tokens {
		if strings.HasPrefix(token.AccessorID, partialID) {
			if tokenID != "" {
				return "", fmt.Errorf("Partial token ID is not unique")
			}
			tokenID = token.AccessorID
		}
	}

	if tokenID == "" {
		return "", fmt.Errorf("No such token ID with prefix: %s", partialID)
	}

	return tokenID, nil
}

func GetPolicyIDFromPartial(client *api.Client, partialID string) (string, error) {
	if partialID == "global-management" {
		return structs.ACLPolicyGlobalManagementID, nil
	}
	// The full UUID string was given
	if len(partialID) == 36 {
		return partialID, nil
	}

	policies, _, err := client.ACL().PolicyList(nil)
	if err != nil {
		return "", err
	}

	policyID := ""
	for _, policy := range policies {
		if strings.HasPrefix(policy.ID, partialID) {
			if policyID != "" {
				return "", fmt.Errorf("Partial policy ID is not unique")
			}
			policyID = policy.ID
		}
	}

	if policyID == "" {
		return "", fmt.Errorf("No such policy ID with prefix: %s", partialID)
	}

	return policyID, nil
}

func GetPolicyIDByName(client *api.Client, name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("No name specified")
	}

	policies, _, err := client.ACL().PolicyList(nil)
	if err != nil {
		return "", err
	}

	for _, policy := range policies {
		if policy.Name == name {
			return policy.ID, nil
		}
	}

	return "", fmt.Errorf("No such policy with name %s", name)
}

func GetRulesFromLegacyToken(client *api.Client, tokenID string, isSecret bool) (string, error) {
	tokenID, err := GetTokenIDFromPartial(client, tokenID)
	if err != nil {
		return "", err
	}

	var token *api.ACLToken
	if isSecret {
		qopts := api.QueryOptions{
			Token: tokenID,
		}
		token, _, err = client.ACL().TokenReadSelf(&qopts)
	} else {
		token, _, err = client.ACL().TokenRead(tokenID, nil)
	}

	if err != nil {
		return "", fmt.Errorf("Error reading token: %v", err)
	}

	if token == nil {
		return "", fmt.Errorf("Token not found for ID")
	}

	if token.Rules == "" {
		return "", fmt.Errorf("Token is not a legacy token with rules")
	}

	return token.Rules, nil
}
