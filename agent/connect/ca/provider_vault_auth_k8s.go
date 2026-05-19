// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package ca

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/agent/structs"
)

func NewK8sAuthClient(authMethod *structs.VaultAuthMethod) (*VaultAuthClient, error) {
	params := authMethod.Params
	role, ok := params["role"].(string)
	if !ok || strings.TrimSpace(role) == "" {
		return nil, fmt.Errorf("missing 'role' value")
	}
	// don't check for `token_path` as it is optional

	authClient := NewVaultAPIAuthClient(authMethod, "")
	// Note the `jwt` can be passed directly in the authMethod as a Param value
	// is a freeform map in the config where they could hardcode it.
	if legacyCheck(params, "jwt") {
		return authClient, nil
	}

	authClient.LoginDataGen = K8sLoginDataGen
	return authClient, nil
}

// K8sLoginDataGen generates the login data for the Kubernetes auth method
func K8sLoginDataGen(authMethod *structs.VaultAuthMethod) (map[string]any, error) {
	params := authMethod.Params
	role := params["role"].(string)

	// read token from file on path
	tokenPath, ok := params["token_path"].(string)
	if !ok || strings.TrimSpace(tokenPath) == "" {
		tokenPath = defaultK8SServiceAccountTokenPath
	}

	// Define allowed base directories for Kubernetes service account tokens
	allowedDirs := []string{
		"/var/run/secrets/kubernetes.io/serviceaccount",
		"/run/secrets/kubernetes.io/serviceaccount",
	}

	// Securely read the JWT file using os.OpenRoot to prevent path traversal attacks
	rawToken, err := readVaultCredentialFileSecurely(tokenPath, allowedDirs)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"role": role,
		"jwt":  strings.TrimSpace(string(rawToken)),
	}, nil
}
