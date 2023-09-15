// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package ca

import (
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/consul/agent/structs"
)

func NewJwtAuthClient(authMethod *structs.VaultAuthMethod) (*VaultAuthClient, error) {
	params := authMethod.Params

	role, ok := params["role"].(string)
	if !ok || strings.TrimSpace(role) == "" {
		return nil, fmt.Errorf("missing 'role' value")
	}

	authClient := NewVaultAPIAuthClient(authMethod, "")
	if legacyCheck(params, "jwt") {
		return authClient, nil
	}

	// The path is required for the auto-auth config, but this auth provider
	// seems to be used for jwt based auth by directly passing the jwt token.
	// So we only require the token file path if the token string isn't
	// present.
	tokenPath, ok := params["path"].(string)
	if !ok || strings.TrimSpace(tokenPath) == "" {
		return nil, fmt.Errorf("missing 'path' value")
	}
	authClient.LoginDataGen = JwtLoginDataGen
	return authClient, nil
}

func JwtLoginDataGen(authMethod *structs.VaultAuthMethod) (map[string]any, error) {
	params := authMethod.Params
	role := params["role"].(string)

	tokenPath := params["path"].(string)
	rawToken, err := os.ReadFile(tokenPath)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"role": role,
		"jwt":  strings.TrimSpace(string(rawToken)),
	}, nil
}
