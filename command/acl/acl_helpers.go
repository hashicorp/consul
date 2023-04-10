// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package acl

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
)

func GetTokenAccessorIDFromPartial(client *api.Client, partialAccessorID string) (string, error) {
	if partialAccessorID == "anonymous" {
		return acl.AnonymousTokenID, nil
	}

	// the full UUID string was given
	if len(partialAccessorID) == 36 {
		return partialAccessorID, nil
	}

	tokens, _, err := client.ACL().TokenList(nil)
	if err != nil {
		return "", err
	}

	tokenAccessorID := ""
	for _, token := range tokens {
		if strings.HasPrefix(token.AccessorID, partialAccessorID) {
			if tokenAccessorID != "" {
				return "", fmt.Errorf("Partial token ID is not unique")
			}
			tokenAccessorID = token.AccessorID
		}
	}

	if tokenAccessorID == "" {
		return "", fmt.Errorf("No such token ID with prefix: %s", partialAccessorID)
	}

	return tokenAccessorID, nil
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

func GetPolicyByName(client *api.Client, name string) (*api.ACLPolicy, error) {
	if name == "" {
		return nil, fmt.Errorf("No name specified")
	}

	policy, _, err := client.ACL().PolicyReadByName(name, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to find policy with name %s: %w", name, err)
	}

	return policy, nil
}

func GetPolicyIDByName(client *api.Client, name string) (string, error) {
	policy, err := GetPolicyByName(client, name)
	if err != nil {
		return "", err
	}

	return policy.ID, nil
}

func GetRoleIDFromPartial(client *api.Client, partialID string) (string, error) {
	// the full UUID string was given
	if len(partialID) == 36 {
		return partialID, nil
	}

	roles, _, err := client.ACL().RoleList(nil)
	if err != nil {
		return "", err
	}

	roleID := ""
	for _, role := range roles {
		if strings.HasPrefix(role.ID, partialID) {
			if roleID != "" {
				return "", fmt.Errorf("Partial role ID is not unique")
			}
			roleID = role.ID
		}
	}

	if roleID == "" {
		return "", fmt.Errorf("No such role ID with prefix: %s", partialID)
	}

	return roleID, nil
}

func GetRoleIDByName(client *api.Client, name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("No name specified")
	}

	roles, _, err := client.ACL().RoleList(nil)
	if err != nil {
		return "", err
	}

	for _, role := range roles {
		if role.Name == name {
			return role.ID, nil
		}
	}

	return "", fmt.Errorf("No such role with name %s", name)
}

func GetBindingRuleIDFromPartial(client *api.Client, partialID string) (string, error) {
	// the full UUID string was given
	if len(partialID) == 36 {
		return partialID, nil
	}

	rules, _, err := client.ACL().BindingRuleList("", nil)
	if err != nil {
		return "", err
	}

	ruleID := ""
	for _, rule := range rules {
		if strings.HasPrefix(rule.ID, partialID) {
			if ruleID != "" {
				return "", fmt.Errorf("Partial rule ID is not unique")
			}
			ruleID = rule.ID
		}
	}

	if ruleID == "" {
		return "", fmt.Errorf("no such rule ID with prefix: %s: %w", partialID, acl.ErrNotFound)
	}

	return ruleID, nil
}

func ExtractServiceIdentities(serviceIdents []string) ([]*api.ACLServiceIdentity, error) {
	var out []*api.ACLServiceIdentity
	for _, svcidRaw := range serviceIdents {
		parts := strings.Split(svcidRaw, ":")
		switch len(parts) {
		case 2:
			out = append(out, &api.ACLServiceIdentity{
				ServiceName: parts[0],
				Datacenters: strings.Split(parts[1], ","),
			})
		case 1:
			out = append(out, &api.ACLServiceIdentity{
				ServiceName: parts[0],
			})
		default:
			return nil, fmt.Errorf("Malformed -service-identity argument: %q", svcidRaw)
		}
	}
	return out, nil
}

func ExtractNodeIdentities(nodeIdents []string) ([]*api.ACLNodeIdentity, error) {
	var out []*api.ACLNodeIdentity
	for _, nodeidRaw := range nodeIdents {
		parts := strings.Split(nodeidRaw, ":")
		switch len(parts) {
		case 2:
			out = append(out, &api.ACLNodeIdentity{
				NodeName:   parts[0],
				Datacenter: parts[1],
			})
		default:
			return nil, fmt.Errorf("Malformed -node-identity argument: %q", nodeidRaw)
		}
	}
	return out, nil
}

// TestKubernetesJWT_A is a valid service account jwt extracted from a minikube setup.
//
//	{
//	  "iss": "kubernetes/serviceaccount",
//	  "kubernetes.io/serviceaccount/namespace": "default",
//	  "kubernetes.io/serviceaccount/secret.name": "admin-token-qlz42",
//	  "kubernetes.io/serviceaccount/service-account.name": "admin",
//	  "kubernetes.io/serviceaccount/service-account.uid": "738bc251-6532-11e9-b67f-48e6c8b8ecb5",
//	  "sub": "system:serviceaccount:default:admin"
//	}
const TestKubernetesJWT_A = "eyJhbGciOiJSUzI1NiIsImtpZCI6IiJ9.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJkZWZhdWx0Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZWNyZXQubmFtZSI6ImFkbWluLXRva2VuLXFsejQyIiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZXJ2aWNlLWFjY291bnQubmFtZSI6ImFkbWluIiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZXJ2aWNlLWFjY291bnQudWlkIjoiNzM4YmMyNTEtNjUzMi0xMWU5LWI2N2YtNDhlNmM4YjhlY2I1Iiwic3ViIjoic3lzdGVtOnNlcnZpY2VhY2NvdW50OmRlZmF1bHQ6YWRtaW4ifQ.ixMlnWrAG7NVuTTKu8cdcYfM7gweS3jlKaEsIBNGOVEjPE7rtXtgMkAwjQTdYR08_0QBjkgzy5fQC5ZNyglSwONJ-bPaXGvhoH1cTnRi1dz9H_63CfqOCvQP1sbdkMeRxNTGVAyWZT76rXoCUIfHP4LY2I8aab0KN9FTIcgZRF0XPTtT70UwGIrSmRpxW38zjiy2ymWL01cc5VWGhJqVysmWmYk3wNp0h5N57H_MOrz4apQR4pKaamzskzjLxO55gpbmZFC76qWuUdexAR7DT2fpbHLOw90atN_NlLMY-VrXyW3-Ei5EhYaVreMB9PSpKwkrA4jULITohV-sxpa1LA"

// TestKubernetesJWT_B is a valid service account jwt extracted from a minikube setup.
//
// {
// "iss": "kubernetes/serviceaccount",
// "kubernetes.io/serviceaccount/namespace": "default",
// "kubernetes.io/serviceaccount/secret.name": "demo-token-kmb9n",
// "kubernetes.io/serviceaccount/service-account.name": "demo",
// "kubernetes.io/serviceaccount/service-account.uid": "76091af4-4b56-11e9-ac4b-708b11801cbe",
// "sub": "system:serviceaccount:default:demo"
// }
const TestKubernetesJWT_B = "eyJhbGciOiJSUzI1NiIsImtpZCI6IiJ9.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJkZWZhdWx0Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZWNyZXQubmFtZSI6ImRlbW8tdG9rZW4ta21iOW4iLCJrdWJlcm5ldGVzLmlvL3NlcnZpY2VhY2NvdW50L3NlcnZpY2UtYWNjb3VudC5uYW1lIjoiZGVtbyIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VydmljZS1hY2NvdW50LnVpZCI6Ijc2MDkxYWY0LTRiNTYtMTFlOS1hYzRiLTcwOGIxMTgwMWNiZSIsInN1YiI6InN5c3RlbTpzZXJ2aWNlYWNjb3VudDpkZWZhdWx0OmRlbW8ifQ.ZiAHjijBAOsKdum0Aix6lgtkLkGo9_Tu87dWQ5Zfwnn3r2FejEWDAnftTft1MqqnMzivZ9Wyyki5ZjQRmTAtnMPJuHC-iivqY4Wh4S6QWCJ1SivBv5tMZR79t5t8mE7R1-OHwst46spru1pps9wt9jsA04d3LpV0eeKYgdPTVaQKklxTm397kIMUugA6yINIBQ3Rh8eQqBgNwEmL4iqyYubzHLVkGkoP9MJikFI05vfRiHtYr-piXz6JFDzXMQj9rW6xtMmrBSn79ChbyvC5nz-Nj2rJPnHsb_0rDUbmXY5PpnMhBpdSH-CbZ4j8jsiib6DtaGJhVZeEQ1GjsFAZwQ"
