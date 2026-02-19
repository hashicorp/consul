// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package ca

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/vault/api"

	"github.com/hashicorp/consul/agent/structs"
)

// VaultAuthenticator defines the interface for logging into Vault using an auth method.
type VaultAuthenticator interface {
	// Login to Vault and return a Vault token.
	Login(ctx context.Context, client *api.Client) (*api.Secret, error)
}

// LoginDataGenerator is used to generate the login data for a Vault login API request.
type LoginDataGeneratorFn func(authMethod *structs.VaultAuthMethod) (map[string]any, error)

var _ VaultAuthenticator = (*VaultAuthClient)(nil)

// VaultAuthClient is a VaultAuthenticator that logs into Vault through the /auth/<method>/login API.
type VaultAuthClient struct {
	// AuthMethod holds the configuration for the Vault auth method login.
	AuthMethod *structs.VaultAuthMethod
	// LoginPath is optional and can be used to explicitly set the API path that the client
	// will use for a login request. If it is empty the path will be derived from AuthMethod.MountPath.
	LoginPath string
	// LoginDataGen derives the parameter map containing the data for the login API request.
	// For some auth methods this is needed to transform the AuthMethod.Params into the login data
	// used for the API request.
	LoginDataGen LoginDataGeneratorFn
}

// NewVaultAPIAuthClient creates a VaultAuthClient that uses the Vault API to perform a login.
func NewVaultAPIAuthClient(authMethod *structs.VaultAuthMethod, loginPath string) *VaultAuthClient {
	if loginPath == "" {
		loginPath = fmt.Sprintf("auth/%s/login", authMethod.MountPath)
	}
	return &VaultAuthClient{
		AuthMethod: authMethod,
		LoginPath:  loginPath,
	}
}

// Login performs a Vault login operation and returns the associated Vault token.
func (c *VaultAuthClient) Login(ctx context.Context, client *api.Client) (*api.Secret, error) {
	var err error
	loginData := c.AuthMethod.Params

	// If a login data generator is provided then use it to transform the auth method params
	// into the appropriate login data for the API request.
	if c.LoginDataGen != nil {
		loginData, err = c.LoginDataGen(c.AuthMethod)
		if err != nil {
			return nil, fmt.Errorf("aws auth failed to generate login data: %w", err)
		}
	}

	if ctx == nil {
		ctx = context.Background()
	}
	return client.Logical().WriteWithContext(ctx, c.LoginPath, loginData)
}

func toMapStringString(in map[string]interface{}) (map[string]string, error) {
	out := make(map[string]string, len(in))
	for k, v := range in {
		if s, ok := v.(string); ok {
			out[k] = s
		} else {
			return nil, fmt.Errorf("invalid type for field %s: expected a string but got %T", k, v)
		}
	}
	return out, nil
}

// legacyCheck is used to see if all the parameters needed to /login have been
// hardcoded in the auth-method's config Parameters field.
// Note it returns true if any /login specific fields are found (vs. all). This
// is because the AWS client has multiple possible ways to call /login with
// different parameters.
func legacyCheck(params map[string]any, expectedKeys ...string) bool {
	for _, key := range expectedKeys {
		if v, ok := params[key]; ok && v != "" {
			return true
		}
	}
	return false
}

// readVaultCredentialFileSecurely reads a Vault credential file using os.OpenRoot to prevent
// path traversal and symlink attacks. This provides OS-level enforcement of file system boundaries.
//
// Parameters:
//   - filePath: the path to the credential file
//   - allowedDirs: a list of allowed base directories where credential files can reside
//
// Returns the file contents or an error if the file is outside allowed directories or cannot be read.
func readVaultCredentialFileSecurely(filePath string, allowedDirs []string) ([]byte, error) {
	// Clean and normalize the input path to remove . and .. elements
	cleanPath := filepath.Clean(filePath)

	// Determine which allowed directory contains the path
	var baseDir string
	var relPath string

	for _, dir := range allowedDirs {
		// Clean the allowed directory path as well
		cleanDir := filepath.Clean(dir)

		// Use filepath.Rel to properly determine if path is within this directory
		rel, err := filepath.Rel(cleanDir, cleanPath)
		if err != nil {
			// filepath.Rel failed, skip this directory
			continue
		}

		// If the relative path starts with "..", the path is outside this directory
		// If it equals ".", the path is the directory itself (not a file)
		// Otherwise, it's a file/subdirectory within the allowed directory
		if !strings.HasPrefix(rel, "..") && rel != "." {
			baseDir = cleanDir
			relPath = rel
			break
		}
	}

	// If no allowed directory matches, reject the path
	if baseDir == "" {
		return nil, fmt.Errorf("credential file must be within allowed directories")
	}

	// Use os.OpenRoot to create a rooted file system restricted to the base directory.
	// This provides OS-level protection against symlink escapes and directory traversal,
	// as any symlinks within the rooted filesystem cannot escape the root boundary.
	root, err := os.OpenRoot(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open root directory")
	}
	defer root.Close()

	// Read the credential file within the rooted file system
	// ReadFile will fail appropriately for directories or special files
	fileBytes, err := root.ReadFile(relPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read credential file")
	}

	// Validate file size to prevent DoS attacks
	if len(fileBytes) > 5*1024*1024 { // 5 MB
		return nil, fmt.Errorf("credential file exceeds maximum allowed 5MB size")
	}

	return fileBytes, nil
}
