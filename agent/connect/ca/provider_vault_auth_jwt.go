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

	// Securely read the JWT token using os.OpenRoot to prevent path traversal attacks
	rawToken, err := readJWTTokenSecurely(tokenPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read JWT token: %w", err)
	}

	return map[string]any{
		"role": role,
		"jwt":  strings.TrimSpace(string(rawToken)),
	}, nil
}

// readJWTTokenSecurely reads a JWT token file using os.OpenRoot to prevent path traversal
// and symlink attacks. This provides OS-level enforcement of file system boundaries.
func readJWTTokenSecurely(tokenPath string) ([]byte, error) {
	// Define allowed base directories for JWT tokens
	allowedDirs := []string{
		"/var/run/secrets/kubernetes.io/serviceaccount",
		"/run/secrets/kubernetes.io/serviceaccount",
		"/var/run/secrets",
		"/run/secrets",
	}

	// Determine which allowed directory contains the token path
	var baseDir string
	var relPath string

	for _, dir := range allowedDirs {
		if strings.HasPrefix(tokenPath, dir+"/") || tokenPath == dir {
			baseDir = dir
			relPath = strings.TrimPrefix(tokenPath, dir+"/")
			if relPath == "" {
				relPath = "token"
			}
			break
		}
	}

	// If no allowed directory matches, reject the path
	if baseDir == "" {
		return nil, fmt.Errorf("JWT token path must be within allowed directories: %v, got: %s", allowedDirs, tokenPath)
	}

	// Use os.OpenRoot to create a rooted file system restricted to the base directory
	root, err := os.OpenRoot(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open root directory %s: %w", baseDir, err)
	}
	defer root.Close()

	// Open the token file within the rooted file system
	file, err := root.Open(relPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open JWT token file: %w", err)
	}
	defer file.Close()

	// Read the token content
	tokenBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read JWT token content: %w", err)
	}

	return tokenBytes, nil
}
