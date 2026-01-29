// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package ca

import (
	"bytes"
	"fmt"
	"io"
	"os"
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

	// Securely read the role_id file using os.OpenRoot to prevent path traversal attacks
	if rawRoleID, err = readAppRoleFileSecurely(roleIdFilePath); err != nil {
		return nil, fmt.Errorf("failed to read role_id file: %w", err)
	}
	data["role_id"] = string(rawRoleID)

	if hasSecret {
		// Securely read the secret_id file using os.OpenRoot to prevent path traversal attacks
		switch rawSecretID, err = readAppRoleFileSecurely(secretIdFilePath); {
		case err != nil:
			return nil, fmt.Errorf("failed to read secret_id file: %w", err)
		case len(bytes.TrimSpace(rawSecretID)) > 0:
			data["secret_id"] = strings.TrimSpace(string(rawSecretID))
		}
	}

	return data, nil
}

// readAppRoleFileSecurely reads an AppRole credential file using os.OpenRoot to prevent
// path traversal and symlink attacks. This provides OS-level enforcement of file system boundaries.
func readAppRoleFileSecurely(filePath string) ([]byte, error) {
	// Define allowed base directories for AppRole credentials
	allowedDirs := []string{
		"/var/run/secrets/vault",
		"/run/secrets/vault",
		"/var/run/secrets",
		"/run/secrets",
	}

	// Determine which allowed directory contains the file path
	var baseDir string
	var relPath string

	for _, dir := range allowedDirs {
		if strings.HasPrefix(filePath, dir+"/") {
			baseDir = dir
			relPath = strings.TrimPrefix(filePath, dir+"/")
			break
		}
	}

	// If no allowed directory matches, reject the path
	if baseDir == "" {
		return nil, fmt.Errorf("AppRole credential file must be within allowed directories: %v, got: %s", allowedDirs, filePath)
	}

	// Use os.OpenRoot to create a rooted file system restricted to the base directory
	root, err := os.OpenRoot(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open root directory %s: %w", baseDir, err)
	}
	defer root.Close()

	// Open the credential file within the rooted file system
	file, err := root.Open(relPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open AppRole credential file: %w", err)
	}
	defer file.Close()

	// Read the file content
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read AppRole credential content: %w", err)
	}

	return fileBytes, nil
}
