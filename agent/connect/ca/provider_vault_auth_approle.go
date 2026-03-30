// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package ca

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/hashicorp/consul/agent/structs"
)

// left out 2 config options as we are re-using vault agent's auth config.
// Why?
// remove_secret_id_file_after_reading - don't remove what we don't own
// secret_id_response_wrapping_path - wrapping the secret before writing to disk
//                                    (which we don't need to do)

func NewAppRoleAuthClient(authMethod *structs.VaultAuthMethod) (*VaultAuthClient, error) {
	authClient := NewVaultAPIAuthClient(authMethod, "")
	// check for hardcoded /login params
	if legacyCheck(authMethod.Params, "role_id", "secret_id") {
		return authClient, nil
	}

	// check for required config params
	key := "role_id_file_path"
	if val, ok := authMethod.Params[key].(string); !ok {
		return nil, fmt.Errorf("missing '%s' value", key)
	} else if strings.TrimSpace(val) == "" {
		return nil, fmt.Errorf("'%s' value is empty", key)
	}
	authClient.LoginDataGen = ArLoginDataGen

	return authClient, nil
}

// ArLoginDataGen generates the login data for the AppRole auth method
func ArLoginDataGen(authMethod *structs.VaultAuthMethod) (map[string]any, error) {
	// don't need to check for legacy params as this func isn't used in that case
	params := authMethod.Params
	// role_id is required
	roleIdFilePath := params["role_id_file_path"].(string)
	// secret_id is optional (secret_ok is used in check below)
	// secretIdFilePath, secret_ok := params["secret_id_file_path"].(string)
	secretIdFilePath, hasSecret := params["secret_id_file_path"].(string)
	if hasSecret && strings.TrimSpace(secretIdFilePath) == "" {
		hasSecret = false
	}

	var err error
	var rawRoleID, rawSecretID []byte
	data := make(map[string]any)

	// Define allowed base directories for AppRole credentials
	allowedDirs := []string{
		"/var/run/secrets/vault",
		"/run/secrets/vault",
		"/var/run/secrets",
		"/run/secrets",
	}

	// Securely read the role_id file using os.OpenRoot to prevent path traversal attacks
	if rawRoleID, err = readVaultCredentialFileSecurely(roleIdFilePath, allowedDirs); err != nil {
		return nil, err
	}
	// Trim whitespace for consistency with secret_id handling
	data["role_id"] = strings.TrimSpace(string(rawRoleID))

	if hasSecret {
		// Securely read the secret_id file using os.OpenRoot to prevent path traversal attacks
		switch rawSecretID, err = readVaultCredentialFileSecurely(secretIdFilePath, allowedDirs); {
		case err != nil:
			return nil, err
		case len(bytes.TrimSpace(rawSecretID)) > 0:
			data["secret_id"] = strings.TrimSpace(string(rawSecretID))
		}
	}

	return data, nil
}
