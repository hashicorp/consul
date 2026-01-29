// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package ca

import (
	"fmt"
	"io"
	"os"
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

func K8sLoginDataGen(authMethod *structs.VaultAuthMethod) (map[string]any, error) {
	params := authMethod.Params
	role := params["role"].(string)

	// read token from file on path
	tokenPath, ok := params["token_path"].(string)
	if !ok || strings.TrimSpace(tokenPath) == "" {
		tokenPath = defaultK8SServiceAccountTokenPath
	}

	// Securely read the token using os.OpenRoot to prevent path traversal attacks.
	// This restricts file access to allowed base directories and prevents symlink escapes.
	rawToken, err := readK8sTokenSecurely(tokenPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read token: %w", err)
	}

	return map[string]any{
		"role": role,
		"jwt":  strings.TrimSpace(string(rawToken)),
	}, nil
}

// readK8sTokenSecurely reads a Kubernetes service account token from a file path
// using os.OpenRoot to prevent path traversal and symlink attacks.
// This provides OS-level enforcement of file system boundaries.
func readK8sTokenSecurely(tokenPath string) ([]byte, error) {
	// Define allowed base directories for Kubernetes service account tokens
	allowedDirs := []string{
		"/var/run/secrets/kubernetes.io/serviceaccount",
		"/run/secrets/kubernetes.io/serviceaccount",
	}

	// Determine which allowed directory contains the token path
	var baseDir string
	var relPath string

	for _, dir := range allowedDirs {
		if strings.HasPrefix(tokenPath, dir+"/") || tokenPath == dir {
			baseDir = dir
			relPath = strings.TrimPrefix(tokenPath, dir+"/")
			if relPath == "" {
				relPath = "token" // default token file name
			}
			break
		}
	}

	// If no allowed directory matches, reject the path
	if baseDir == "" {
		return nil, fmt.Errorf("token_path must be within allowed directories: %v, got: %s", allowedDirs, tokenPath)
	}

	// Use os.OpenRoot to create a rooted file system restricted to the base directory.
	// This prevents path traversal attacks and symlink escapes at the OS level.
	root, err := os.OpenRoot(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open root directory %s: %w", baseDir, err)
	}
	defer root.Close()

	// Open the token file within the rooted file system.
	// Any attempt to escape the base directory will be blocked by the OS.
	file, err := root.Open(relPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open token file: %w", err)
	}
	defer file.Close()

	// Read the token content using io.ReadAll
	tokenBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read token content: %w", err)
	}

	return tokenBytes, nil
}
