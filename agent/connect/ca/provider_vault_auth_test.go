// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package ca

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/vault/api/auth/gcp"
	"github.com/hashicorp/vault/sdk/helper/jsonutil"

	"github.com/hashicorp/consul/agent/structs"
)

func TestVaultCAProvider_GCPAuthClient(t *testing.T) {
	cases := map[string]struct {
		authMethod *structs.VaultAuthMethod
		isExplicit bool
		expErr     error
	}{
		"explicit config": {
			authMethod: &structs.VaultAuthMethod{
				Type: "gcp",
				Params: map[string]interface{}{
					"role": "test-role",
					"jwt":  "test-jwt",
				},
			},
			isExplicit: true,
		},
		"derived iam auth": {
			authMethod: &structs.VaultAuthMethod{
				Type: "gcp",
				Params: map[string]interface{}{
					"type":                  "iam",
					"role":                  "test-role",
					"service_account_email": "test@google.cloud",
				},
			},
		},
		"derived gce auth": {
			authMethod: &structs.VaultAuthMethod{
				Type: "gcp",
				Params: map[string]interface{}{
					"type": "gce",
					"role": "test-role",
				},
			},
		},
		"derived without role": {
			authMethod: &structs.VaultAuthMethod{
				Type: "gcp",
				Params: map[string]interface{}{
					"type": "gce",
				},
			},
			expErr: fmt.Errorf("failed to create a new Vault GCP auth client"),
		},
		"invalid config": {
			authMethod: &structs.VaultAuthMethod{
				Type: "gcp",
				Params: map[string]interface{}{
					"invalid": true,
				},
			},
			expErr: fmt.Errorf("misconfiguration of GCP auth parameters: invalid type for field"),
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			auth, err := NewGCPAuthClient(c.authMethod)
			if c.expErr != nil {
				require.Error(t, err)
				require.Contains(t, err.Error(), c.expErr.Error())
				return
			}

			require.NoError(t, err)
			require.NotNil(t, auth)

			if c.isExplicit {
				// in this case a JWT is provided so we'll call the login API directly using a VaultAuthClient.
				_ = auth.(*VaultAuthClient)
			} else {
				// in this case we delegate to gcp.GCPAuth to perform the login.
				_ = auth.(*gcp.GCPAuth)
			}
		})
	}
}

func TestVaultCAProvider_AWSAuthClient(t *testing.T) {
	cases := map[string]struct {
		authMethod   *structs.VaultAuthMethod
		expLoginPath string
		hasLDG       bool
	}{
		"explicit aws ec2 identity": {
			authMethod: &structs.VaultAuthMethod{
				Type: "aws",
				Params: map[string]interface{}{
					"role":      "test-role",
					"identity":  "test-identity",
					"signature": "test-signature",
				},
			},
			expLoginPath: "auth/aws/login",
		},
		"explicit aws ec2 pkcs7": {
			authMethod: &structs.VaultAuthMethod{
				Type:      "aws",
				MountPath: "custom-aws",
				Params: map[string]interface{}{
					"role":  "test-role",
					"pkcs7": "test-pkcs7",
				},
			},
			expLoginPath: "auth/custom-aws/login",
		},
		"derived aws login data": {
			authMethod: &structs.VaultAuthMethod{
				Type: "aws",
				Params: map[string]interface{}{
					"role":   "test-role",
					"type":   "ec2",
					"region": "test-region",
				},
			},
			expLoginPath: "auth/aws/login",
			hasLDG:       true,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if c.authMethod.MountPath == "" {
				c.authMethod.MountPath = c.authMethod.Type
			}
			auth := NewAWSAuthClient(c.authMethod)
			require.Equal(t, c.authMethod, auth.AuthMethod)
			require.Equal(t, c.expLoginPath, auth.LoginPath)
			if c.hasLDG {
				require.NotNil(t, auth.LoginDataGen)
			} else {
				require.Nil(t, auth.LoginDataGen)
			}
		})
	}
}

func TestVaultCAProvider_AWSCredentialsConfig(t *testing.T) {
	cases := map[string]struct {
		params    map[string]interface{}
		envVars   map[string]string
		expRegion string
		expErr    error
	}{
		"valid config": {
			params: map[string]interface{}{
				"access_key":              "access key",
				"secret_key":              "secret key",
				"region":                  "custom-region",
				"role_arn":                "role arn",
				"role_session_name":       "role session name",
				"web_identity_token_file": "web identity token file",
				"header_value":            "header value",
				"max_retries":             "13",
			},
			expRegion: "custom-region",
		},
		"default region": {
			params:    map[string]interface{}{},
			expRegion: "us-east-1",
		},
		"env AWS_REGION": {
			params:    map[string]interface{}{},
			envVars:   map[string]string{"AWS_REGION": "us-west-1"},
			expRegion: "us-west-1",
		},
		"env AWS_DEFAULT_REGION": {
			params:    map[string]interface{}{},
			envVars:   map[string]string{"AWS_DEFAULT_REGION": "us-west-2"},
			expRegion: "us-west-2",
		},
		"both AWS_REGION and AWS_DEFAULT_REGION": {
			params: map[string]interface{}{},
			envVars: map[string]string{
				"AWS_REGION":         "us-west-1",
				"AWS_DEFAULT_REGION": "us-west-2",
			},
			expRegion: "us-west-1",
		},
		"with role_external_id": {
			params: map[string]interface{}{
				"access_key":       "access key",
				"secret_key":       "secret key",
				"region":           "eu-west-1",
				"role_arn":         "arn:aws:iam::123456789012:role/test-role",
				"role_external_id": "external-id-12345",
			},
			expRegion: "eu-west-1",
		},
		"with web_identity_token": {
			params: map[string]interface{}{
				"region":             "ap-southeast-1",
				"role_arn":           "arn:aws:iam::123456789012:role/web-identity-role",
				"web_identity_token": "mock-web-identity-token",
				"role_session_name":  "test-session",
			},
			expRegion: "ap-southeast-1",
		},
		"invalid config": {
			params: map[string]interface{}{
				"invalid": true,
			},
			expErr: fmt.Errorf("misconfiguration of AWS auth parameters: invalid type for field"),
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if c.envVars != nil {
				for k, v := range c.envVars {
					require.NoError(t, os.Setenv(k, v))
				}
				t.Cleanup(func() {
					for k := range c.envVars {
						os.Unsetenv(k)
					}
				})
			}
			creds, headerValue, err := newAWSCredentialsConfig(c.params)
			if c.expErr != nil {
				require.Error(t, err)
				require.Contains(t, err.Error(), c.expErr.Error())
				return
			}

			require.NoError(t, err)

			// If a header value was provided in the params then make sure it was returned.
			if val, ok := c.params["header_value"]; ok {
				require.Equal(t, val, headerValue)
			} else {
				require.Empty(t, headerValue)
			}

			// Verify the credentials config was created successfully by generating a config
			ctx := context.Background()
			awsConfig, err := creds.GenerateCredentialChain(ctx)
			require.NoError(t, err)
			require.NotNil(t, awsConfig)
			require.Equal(t, c.expRegion, awsConfig.Region)
		})
	}
}

func TestVaultCAProvider_AWSLoginDataGenerator(t *testing.T) {
	cases := map[string]struct {
		expErr        error
		authMethod    structs.VaultAuthMethod
		useNilConfig  bool
		expectHeaders bool
	}{
		"valid login data": {
			authMethod: structs.VaultAuthMethod{},
		},
		"with role": {
			expErr:     nil,
			authMethod: structs.VaultAuthMethod{Type: "aws", MountPath: "", Params: map[string]interface{}{"role": "test-role"}},
		},
		"with header_value": {
			authMethod: structs.VaultAuthMethod{
				Type:      "aws",
				MountPath: "",
				Params: map[string]interface{}{
					"access_key":   "AKIAIOSFODNN7EXAMPLE",
					"secret_key":   "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
					"region":       "us-west-2",
					"header_value": "vault.example.com",
				},
			},
			useNilConfig:  true,
			expectHeaders: true,
		},
		"nil awsConfig generates credentials": {
			authMethod: structs.VaultAuthMethod{
				Type:      "aws",
				MountPath: "",
				Params: map[string]interface{}{
					"access_key": "AKIAIOSFODNN7EXAMPLE",
					"secret_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
					"region":     "eu-central-1",
				},
			},
			useNilConfig: true,
		},
		"error on invalid config": {
			authMethod: structs.VaultAuthMethod{
				Type:      "aws",
				MountPath: "",
				Params: map[string]interface{}{
					"invalid": true,
				},
			},
			expErr: fmt.Errorf("aws auth failed to create credential configuration"),
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			var ldg *AWSLoginDataGenerator

			if c.useNilConfig {
				// Test the path where awsConfig is nil and needs to be generated
				ldg = &AWSLoginDataGenerator{awsConfig: nil}
			} else {
				// Create a mock AWS config with anonymous credentials for testing
				mockConfig := &aws.Config{
					Region: "us-east-1",
					Credentials: aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
						return aws.Credentials{
							AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
							SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
						}, nil
					}),
				}
				ldg = &AWSLoginDataGenerator{awsConfig: mockConfig}
			}

			loginData, err := ldg.GenerateLoginData(&c.authMethod)
			if c.expErr != nil {
				require.Error(t, err)
				require.Contains(t, err.Error(), c.expErr.Error())
				return
			}

			require.NoError(t, err)

			keys := []string{
				"iam_http_request_method",
				"iam_request_url",
				"iam_request_headers",
				"iam_request_body",
			}

			for _, key := range keys {
				val, exists := loginData[key]
				require.True(t, exists, "missing expected key: %s", key)
				require.NotEmpty(t, val, "expected non-empty value for key: %s", key)
			}

			if c.authMethod.Params["role"] != nil {
				require.Equal(t, c.authMethod.Params["role"], loginData["role"])
			}
		})
	}
}

// TestVaultCAProvider_AWSEndpointConfiguration tests that custom IAM and STS endpoints
// are properly extracted from auth method params and passed to consul-awsauth.
func TestVaultCAProvider_AWSEndpointConfiguration(t *testing.T) {
	cases := map[string]struct {
		params              map[string]interface{}
		expectLoginDataKeys []string
		expectRole          string
	}{
		"with custom iam_endpoint": {
			params: map[string]interface{}{
				"access_key":   "AKIAIOSFODNN7EXAMPLE",
				"secret_key":   "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
				"region":       "us-east-1",
				"iam_endpoint": "https://iam.custom-endpoint.example.com",
			},
			expectLoginDataKeys: []string{
				"iam_http_request_method",
				"iam_request_url",
				"iam_request_headers",
				"iam_request_body",
			},
		},
		"with custom sts_endpoint": {
			params: map[string]interface{}{
				"access_key":   "AKIAIOSFODNN7EXAMPLE",
				"secret_key":   "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
				"region":       "us-west-2",
				"sts_endpoint": "https://sts.custom-endpoint.example.com",
			},
			expectLoginDataKeys: []string{
				"iam_http_request_method",
				"iam_request_url",
				"iam_request_headers",
				"iam_request_body",
			},
		},
		"with both iam_endpoint and sts_endpoint": {
			params: map[string]interface{}{
				"access_key":   "AKIAIOSFODNN7EXAMPLE",
				"secret_key":   "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
				"region":       "eu-west-1",
				"iam_endpoint": "https://iam.custom.example.com",
				"sts_endpoint": "https://sts.custom.example.com",
			},
			expectLoginDataKeys: []string{
				"iam_http_request_method",
				"iam_request_url",
				"iam_request_headers",
				"iam_request_body",
			},
		},
		"without custom endpoints (default behavior)": {
			params: map[string]interface{}{
				"access_key": "AKIAIOSFODNN7EXAMPLE",
				"secret_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
				"region":     "ap-southeast-1",
			},
			expectLoginDataKeys: []string{
				"iam_http_request_method",
				"iam_request_url",
				"iam_request_headers",
				"iam_request_body",
			},
		},
		"with role and custom endpoints": {
			params: map[string]interface{}{
				"access_key":   "AKIAIOSFODNN7EXAMPLE",
				"secret_key":   "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
				"region":       "us-east-1",
				"role":         "test-vault-role",
				"iam_endpoint": "https://iam-fips.us-gov-west-1.amazonaws.com",
				"sts_endpoint": "https://sts-fips.us-gov-west-1.amazonaws.com",
			},
			expectLoginDataKeys: []string{
				"iam_http_request_method",
				"iam_request_url",
				"iam_request_headers",
				"iam_request_body",
			},
			expectRole: "test-vault-role",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Create auth method with params
			authMethod := &structs.VaultAuthMethod{
				Type:      "aws",
				MountPath: "aws",
				Params:    c.params,
			}

			// Create AWS auth client which initializes the login data generator
			authClient := NewAWSAuthClient(authMethod)
			require.NotNil(t, authClient)
			require.NotNil(t, authClient.LoginDataGen)

			// Generate login data - this tests the actual code path in provider_vault_auth_aws.go
			// including extraction of iam_endpoint and sts_endpoint from params
			loginData, err := authClient.LoginDataGen(authMethod)
			require.NoError(t, err)
			require.NotNil(t, loginData)

			// Verify that all expected login data keys are present
			for _, key := range c.expectLoginDataKeys {
				val, exists := loginData[key]
				require.True(t, exists, "missing expected key: %s", key)
				require.NotEmpty(t, val, "expected non-empty value for key: %s", key)
			}

			// If role was specified, verify it's in the login data
			if c.expectRole != "" {
				require.Equal(t, c.expectRole, loginData["role"])
			}

			// Note: We can't directly verify that the endpoints were passed to consul-awsauth
			// without mocking the IAM/STS services, but we can verify:
			// 1. The function succeeds (doesn't error)
			// 2. Login data is generated with all required fields
			// 3. The code path that extracts iam_endpoint and sts_endpoint executes

			// The actual endpoint usage is tested in consul-awsauth's own test suite
			// This test verifies the integration: extracting params and calling GenerateLoginData
		})
	}
}

func TestVaultCAProvider_AzureAuthClient(t *testing.T) {
	instance := instanceData{Compute: Compute{
		Name: "a", ResourceGroupName: "b", SubscriptionID: "c", VMScaleSetName: "d",
	}}
	instanceJSON, err := json.Marshal(instance)
	require.NoError(t, err)
	identity := identityData{AccessToken: "a-jwt-token"}
	identityJSON, err := json.Marshal(identity)
	require.NoError(t, err)

	msi := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			url := r.URL.Path
			switch url {
			case "/metadata/instance":
				w.Write(instanceJSON)
			case "/metadata/identity/oauth2/token":
				w.Write(identityJSON)
			default:
				t.Errorf("unexpected testing URL: %s", url)
			}
		}))

	origIn, origId := instanceEndpoint, identityEndpoint
	instanceEndpoint = msi.URL + "/metadata/instance"
	identityEndpoint = msi.URL + "/metadata/identity/oauth2/token"
	defer func() {
		instanceEndpoint, identityEndpoint = origIn, origId
	}()

	t.Run("get-metadata-instance-info", func(t *testing.T) {
		md, err := getMetadataInfo(instanceEndpoint, nil)
		require.NoError(t, err)
		var testInstance instanceData
		err = jsonutil.DecodeJSON(md, &testInstance)
		require.NoError(t, err)
		require.Equal(t, testInstance, instance)
	})

	t.Run("get-metadata-identity-info", func(t *testing.T) {
		md, err := getMetadataInfo(identityEndpoint, nil)
		require.NoError(t, err)
		var testIdentity identityData
		err = jsonutil.DecodeJSON(md, &testIdentity)
		require.NoError(t, err)
		require.Equal(t, testIdentity, identity)
	})

	cases := map[string]struct {
		authMethod *structs.VaultAuthMethod
		expData    map[string]any
		expErr     error
	}{
		"legacy-case": {
			authMethod: &structs.VaultAuthMethod{
				Type: "azure",
				Params: map[string]interface{}{
					"role":                "a",
					"vm_name":             "b",
					"vmss_name":           "c",
					"resource_group_name": "d",
					"subscription_id":     "e",
					"jwt":                 "f",
				},
			},
			expData: map[string]any{
				"role":                "a",
				"vm_name":             "b",
				"vmss_name":           "c",
				"resource_group_name": "d",
				"subscription_id":     "e",
				"jwt":                 "f",
			},
		},
		"base-case": {
			authMethod: &structs.VaultAuthMethod{
				Type: "azure",
				Params: map[string]interface{}{
					"role":     "a-role",
					"resource": "b-resource",
				},
			},
			expData: map[string]any{
				"role": "a-role",
				"jwt":  "a-jwt-token",
			},
		},
		"no-role": {
			authMethod: &structs.VaultAuthMethod{
				Type: "azure",
				Params: map[string]interface{}{
					"resource": "b-resource",
				},
			},
			expErr: fmt.Errorf("missing 'role' value"),
		},
		"no-resource": {
			authMethod: &structs.VaultAuthMethod{
				Type: "azure",
				Params: map[string]interface{}{
					"role": "a-role",
				},
			},
			expErr: fmt.Errorf("missing 'resource' value"),
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			auth, err := NewAzureAuthClient(c.authMethod)
			if c.expErr != nil {
				require.EqualError(t, err, c.expErr.Error())
				return
			}
			require.NoError(t, err)
			if auth.LoginDataGen != nil {
				data, err := auth.LoginDataGen(c.authMethod)
				require.NoError(t, err)
				require.Subset(t, data, c.expData)
			}
		})
	}
}

func TestVaultCAProvider_JwtAuthClient(t *testing.T) {
	// Create temp directory and symlink from allowed directory
	tmpDir := t.TempDir()
	tokenDir := filepath.Join(tmpDir, "jwt-tokens")
	err := os.MkdirAll(tokenDir, 0755)
	require.NoError(t, err)

	// Create symlink from allowed directory to temp directory
	symlinkPath := "/var/run/secrets"
	_ = os.Remove(symlinkPath) // Remove if exists (ignore error)
	err = os.Symlink(tokenDir, symlinkPath)
	if err != nil {
		t.Skipf("Cannot create symlink %s -> %s (requires permissions): %v", symlinkPath, tokenDir, err)
	}
	defer os.Remove(symlinkPath)

	tokenPath := filepath.Join(symlinkPath, "jwt-token")
	err = os.WriteFile(tokenPath, []byte("test-token"), 0644)
	require.NoError(t, err)

	cases := map[string]struct {
		authMethod *structs.VaultAuthMethod
		expData    map[string]any
		expErr     error
	}{
		"base-case": {
			authMethod: &structs.VaultAuthMethod{
				Type: "jwt",
				Params: map[string]any{
					"role": "test-role",
					"path": tokenPath,
				},
			},
			expData: map[string]any{
				"role": "test-role",
				"jwt":  "test-token",
			},
		},
		"no-role": {
			authMethod: &structs.VaultAuthMethod{
				Type:   "jwt",
				Params: map[string]any{},
			},
			expErr: fmt.Errorf("missing 'role' value"),
		},
		"no-path": {
			authMethod: &structs.VaultAuthMethod{
				Type: "jwt",
				Params: map[string]any{
					"role": "test-role",
				},
			},
			expErr: fmt.Errorf("missing 'path' value"),
		},
		"no-path-but-jwt": {
			authMethod: &structs.VaultAuthMethod{
				Type: "jwt",
				Params: map[string]any{
					"role": "test-role",
					"jwt":  "test-jwt",
				},
			},
			expData: map[string]any{
				"role": "test-role",
				"jwt":  "test-jwt",
			},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			auth, err := NewJwtAuthClient(c.authMethod)
			if c.expErr != nil {
				require.EqualError(t, c.expErr, err.Error())
				return
			}
			require.NoError(t, err)
			if auth.LoginDataGen != nil {
				data, err := auth.LoginDataGen(c.authMethod)
				require.NoError(t, err)
				require.Equal(t, c.expData, data)
			}
		})
	}
}

func TestVaultCAProvider_K8sAuthClient(t *testing.T) {
	// Create temp directory and symlink from allowed directory
	tmpDir := t.TempDir()
	tokenDir := filepath.Join(tmpDir, "serviceaccount")
	err := os.MkdirAll(tokenDir, 0755)
	require.NoError(t, err)

	// Create symlink from allowed directory to temp directory
	symlinkPath := "/var/run/secrets/kubernetes.io/serviceaccount"
	_ = os.RemoveAll(symlinkPath) // Remove if exists (ignore error)
	err = os.MkdirAll(filepath.Dir(symlinkPath), 0755)
	if err != nil {
		t.Skipf("Cannot create parent directory for symlink (requires permissions): %v", err)
	}
	err = os.Symlink(tokenDir, symlinkPath)
	if err != nil {
		t.Skipf("Cannot create symlink %s -> %s (requires permissions): %v", symlinkPath, tokenDir, err)
	}
	defer os.RemoveAll("/var/run/secrets/kubernetes.io")

	tokenPath := filepath.Join(symlinkPath, "token")
	err = os.WriteFile(tokenPath, []byte("test-token"), 0644)
	require.NoError(t, err)

	cases := map[string]struct {
		authMethod *structs.VaultAuthMethod
		expData    map[string]any
		expErr     error
	}{
		"base-case": {
			authMethod: &structs.VaultAuthMethod{
				Type: "kubernetes",
				Params: map[string]any{
					"role":       "test-role",
					"token_path": tokenPath,
				},
			},
			expData: map[string]any{
				"role": "test-role",
				"jwt":  "test-token",
			},
		},
		"legacy-case": {
			authMethod: &structs.VaultAuthMethod{
				Type: "kubernetes",
				Params: map[string]any{
					"role": "test-role",
					"jwt":  "test-token",
				},
			},
			expData: map[string]any{
				"role": "test-role",
				"jwt":  "test-token",
			},
		},
		"no-role": {
			authMethod: &structs.VaultAuthMethod{
				Type:   "kubernetes",
				Params: map[string]any{},
			},
			expErr: fmt.Errorf("missing 'role' value"),
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			auth, err := NewK8sAuthClient(c.authMethod)
			if c.expErr != nil {
				require.Error(t, err)
				require.EqualError(t, c.expErr, err.Error())
				return
			}
			require.NoError(t, err)
			if auth.LoginDataGen != nil {
				data, err := auth.LoginDataGen(c.authMethod)
				require.NoError(t, err)
				require.Equal(t, c.expData, data)
			}
		})
	}
}

func TestVaultCAProvider_AppRoleAuthClient(t *testing.T) {
	roleID, secretID := "test_role_id", "test_secret_id"

	// Create temp directory and symlink from allowed directory
	tmpDir := t.TempDir()
	vaultDir := filepath.Join(tmpDir, "vault")
	err := os.MkdirAll(vaultDir, 0755)
	require.NoError(t, err)

	// Create symlink from allowed directory to temp directory
	symlinkPath := "/var/run/secrets/vault"
	_ = os.RemoveAll(symlinkPath) // Remove if exists (ignore error)
	err = os.MkdirAll(filepath.Dir(symlinkPath), 0755)
	if err != nil {
		t.Skipf("Cannot create parent directory for symlink (requires permissions): %v", err)
	}
	err = os.Symlink(vaultDir, symlinkPath)
	if err != nil {
		t.Skipf("Cannot create symlink %s -> %s (requires permissions): %v", symlinkPath, vaultDir, err)
	}
	defer os.RemoveAll("/var/run/secrets/vault")

	roleIdPath := filepath.Join(symlinkPath, "role-id")
	err = os.WriteFile(roleIdPath, []byte(roleID), 0644)
	require.NoError(t, err)

	secretIdPath := filepath.Join(symlinkPath, "secret-id")
	err = os.WriteFile(secretIdPath, []byte(secretID), 0644)
	require.NoError(t, err)

	cases := map[string]struct {
		authMethod *structs.VaultAuthMethod
		expData    map[string]any
		expErr     error
	}{
		"base-case": {
			authMethod: &structs.VaultAuthMethod{
				Type: "approle",
				Params: map[string]any{
					"role_id_file_path":   roleIdPath,
					"secret_id_file_path": secretIdPath,
				},
			},
			expData: map[string]any{
				"role_id":   roleID,
				"secret_id": secretID,
			},
		},
		"optional-secret-left-out": {
			authMethod: &structs.VaultAuthMethod{
				Type: "approle",
				Params: map[string]any{
					"role_id_file_path": roleIdPath,
				},
			},
			expData: map[string]any{
				"role_id": roleID,
			},
		},
		"missing-role-id-file-path": {
			authMethod: &structs.VaultAuthMethod{
				Type:   "approle",
				Params: map[string]any{},
			},
			expErr: fmt.Errorf("missing '%s' value", "role_id_file_path"),
		},
		"legacy-direct-values": {
			authMethod: &structs.VaultAuthMethod{
				Type: "approle",
				Params: map[string]any{
					"role_id":   "test-role",
					"secret_id": "test-secret",
				},
			},
			expData: map[string]any{
				"role_id":   "test-role",
				"secret_id": "test-secret",
			},
		},
	}

	for k, c := range cases {
		t.Run(k, func(t *testing.T) {
			auth, err := NewAppRoleAuthClient(c.authMethod)
			if c.expErr != nil {
				require.Error(t, err)
				require.EqualError(t, c.expErr, err.Error())
				return
			}
			require.NoError(t, err)
			if auth.LoginDataGen != nil {
				data, err := auth.LoginDataGen(c.authMethod)
				require.NoError(t, err)
				require.Equal(t, c.expData, data)
			}
		})
	}
}

func TestVaultCAProvider_AliCloudAuthClient(t *testing.T) {
	// required as login parameters, will hang if not set
	os.Setenv("ALICLOUD_ACCESS_KEY", "test-access-key")
	os.Setenv("ALICLOUD_SECRET_KEY", "test-secret-key")
	os.Setenv("ALICLOUD_ACCESS_KEY_STS_TOKEN", "test-access-token")
	defer func() {
		os.Unsetenv("ALICLOUD_ACCESS_KEY")
		os.Unsetenv("ALICLOUD_SECRET_KEY")
		os.Unsetenv("ALICLOUD_ACCESS_KEY_STS_TOKEN")
	}()
	cases := map[string]struct {
		authMethod *structs.VaultAuthMethod
		expQry     map[string][]string
		expErr     error
	}{
		"base-case": {
			authMethod: &structs.VaultAuthMethod{
				Type: VaultAuthMethodTypeAliCloud,
				Params: map[string]interface{}{
					"role":   "test-role",
					"region": "test-region",
				},
			},
			expQry: map[string][]string{
				"Action":      {"GetCallerIdentity"},
				"AccessKeyId": {"test-access-key"},
				"RegionId":    {"test-region"},
			},
		},
		"no-role": {
			authMethod: &structs.VaultAuthMethod{
				Type: VaultAuthMethodTypeAliCloud,
				Params: map[string]interface{}{
					"region": "test-region",
				},
			},
			expErr: fmt.Errorf("role is required for AliCloud login"),
		},
		"no-region": {
			authMethod: &structs.VaultAuthMethod{
				Type: VaultAuthMethodTypeAliCloud,
				Params: map[string]interface{}{
					"role": "test-role",
				},
			},
			expErr: fmt.Errorf("region is required for AliCloud login"),
		},
		"legacy-case": {
			authMethod: &structs.VaultAuthMethod{
				Type: VaultAuthMethodTypeAliCloud,
				Params: map[string]interface{}{
					"access_key":   "test-key",
					"access_token": "test-token",
					"secret_key":   "test-secret-key",
				},
			},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			auth, err := NewAliCloudAuthClient(c.authMethod)
			if c.expErr != nil {
				require.Error(t, err)
				require.EqualError(t, c.expErr, err.Error())
				return
			}
			require.NotNil(t, auth)

			if auth.LoginDataGen != nil {
				encodedData, err := auth.LoginDataGen(c.authMethod)
				require.NoError(t, err)

				// identity_request_headers (json encoded headers)
				rawheaders, err := base64.StdEncoding.DecodeString(
					encodedData["identity_request_headers"].(string))
				require.NoError(t, err)
				headers := string(rawheaders)
				require.Contains(t, headers, "User-Agent")
				require.Contains(t, headers, "AlibabaCloud")
				require.Contains(t, headers, "Content-Type")
				require.Contains(t, headers, "x-acs-action")
				require.Contains(t, headers, "GetCallerIdentity")

				// identity_request_url (w/ query params)
				rawurl, err := base64.StdEncoding.DecodeString(
					encodedData["identity_request_url"].(string))
				require.NoError(t, err)
				requrl, err := url.Parse(string(rawurl))
				require.NoError(t, err)

				queries := requrl.Query()
				require.Subset(t, queries, c.expQry, "query missing fields")
				require.Equal(t, requrl.Hostname(), "sts.test-region.aliyuncs.com")
			}
		})
	}
}

func TestReadVaultCredentialFileSecurely(t *testing.T) {
	// Create a temporary directory structure for testing
	tmpDir := t.TempDir()

	allowedDir := filepath.Join(tmpDir, "allowed")
	disallowedDir := filepath.Join(tmpDir, "disallowed")

	require.NoError(t, os.MkdirAll(allowedDir, 0755))
	require.NoError(t, os.MkdirAll(disallowedDir, 0755))

	// Create test files
	allowedFile := filepath.Join(allowedDir, "credential.txt")
	disallowedFile := filepath.Join(disallowedDir, "credential.txt")

	testContent := []byte("test-credential-content")
	require.NoError(t, os.WriteFile(allowedFile, testContent, 0644))
	require.NoError(t, os.WriteFile(disallowedFile, testContent, 0644))

	allowedDirs := []string{allowedDir}

	t.Run("successfully reads file from allowed directory", func(t *testing.T) {
		content, err := readVaultCredentialFileSecurely(allowedFile, allowedDirs)
		require.NoError(t, err)
		require.Equal(t, testContent, content)
	})

	t.Run("rejects file from disallowed directory", func(t *testing.T) {
		_, err := readVaultCredentialFileSecurely(disallowedFile, allowedDirs)
		require.Error(t, err)
		require.Contains(t, err.Error(), "credential file must be within allowed directories")
		// Ensure the error doesn't leak the file path
		require.NotContains(t, err.Error(), disallowedFile)
	})

	t.Run("rejects path traversal attempts", func(t *testing.T) {
		// Try to escape using ../
		traversalPath := filepath.Join(allowedDir, "..", "disallowed", "credential.txt")
		_, err := readVaultCredentialFileSecurely(traversalPath, allowedDirs)
		require.Error(t, err)
	})

	t.Run("rejects absolute path outside allowed directories", func(t *testing.T) {
		_, err := readVaultCredentialFileSecurely("/etc/passwd", allowedDirs)
		require.Error(t, err)
		require.Contains(t, err.Error(), "credential file must be within allowed directories")
	})

	t.Run("handles symlink within allowed directory", func(t *testing.T) {
		// Create a symlink within the allowed directory using relative path (like Kubernetes)
		symlinkPath := filepath.Join(allowedDir, "symlink.txt")
		targetFile := "target.txt"
		targetPath := filepath.Join(allowedDir, targetFile)

		require.NoError(t, os.WriteFile(targetPath, []byte("target-content"), 0644))
		// Use relative symlink (not absolute) - this is how Kubernetes mounts secrets
		require.NoError(t, os.Symlink(targetFile, symlinkPath))

		content, err := readVaultCredentialFileSecurely(symlinkPath, allowedDirs)
		require.NoError(t, err)
		require.Equal(t, []byte("target-content"), content)
	})

	t.Run("rejects symlink pointing outside allowed directory", func(t *testing.T) {
		// Create a symlink inside allowed dir pointing to disallowed dir
		symlinkPath := filepath.Join(allowedDir, "bad-symlink.txt")

		require.NoError(t, os.Symlink(disallowedFile, symlinkPath))

		_, err := readVaultCredentialFileSecurely(symlinkPath, allowedDirs)
		require.Error(t, err)
		// os.OpenRoot() blocks the symlink escape at read time, not validation time
		require.Contains(t, err.Error(), "failed to read credential file")
	})

	t.Run("handles non-existent file", func(t *testing.T) {
		nonExistentFile := filepath.Join(allowedDir, "nonexistent.txt")
		_, err := readVaultCredentialFileSecurely(nonExistentFile, allowedDirs)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to read credential file")
	})

	t.Run("handles multiple allowed directories", func(t *testing.T) {
		secondAllowedDir := filepath.Join(tmpDir, "allowed2")
		require.NoError(t, os.MkdirAll(secondAllowedDir, 0755))

		secondAllowedFile := filepath.Join(secondAllowedDir, "cred2.txt")
		require.NoError(t, os.WriteFile(secondAllowedFile, []byte("cred2"), 0644))

		multiAllowedDirs := []string{allowedDir, secondAllowedDir}

		content, err := readVaultCredentialFileSecurely(secondAllowedFile, multiAllowedDirs)
		require.NoError(t, err)
		require.Equal(t, []byte("cred2"), content)
	})

	t.Run("rejects empty allowed directories list", func(t *testing.T) {
		_, err := readVaultCredentialFileSecurely(allowedFile, []string{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "credential file must be within allowed directories")
	})

	t.Run("handles file with special characters in name", func(t *testing.T) {
		specialFile := filepath.Join(allowedDir, "cred-file_123.json")
		require.NoError(t, os.WriteFile(specialFile, []byte("special"), 0644))

		content, err := readVaultCredentialFileSecurely(specialFile, allowedDirs)
		require.NoError(t, err)
		require.Equal(t, []byte("special"), content)
	})

	// ========== ADDITIONAL TEST CASES ==========

	t.Run("handles nested subdirectories within allowed directory", func(t *testing.T) {
		nestedDir := filepath.Join(allowedDir, "subdir1", "subdir2")
		require.NoError(t, os.MkdirAll(nestedDir, 0755))

		nestedFile := filepath.Join(nestedDir, "nested.txt")
		require.NoError(t, os.WriteFile(nestedFile, []byte("nested-content"), 0644))

		content, err := readVaultCredentialFileSecurely(nestedFile, allowedDirs)
		require.NoError(t, err)
		require.Equal(t, []byte("nested-content"), content)
	})

	t.Run("handles allowed directory with trailing slash", func(t *testing.T) {
		allowedDirsWithSlash := []string{allowedDir + string(filepath.Separator)}

		content, err := readVaultCredentialFileSecurely(allowedFile, allowedDirsWithSlash)
		require.NoError(t, err)
		require.Equal(t, testContent, content)
	})

	t.Run("handles path with redundant separators", func(t *testing.T) {
		// Path like /var/run//secrets///vault/file.txt
		redundantPath := filepath.Join(allowedDir, "redundant.txt")
		require.NoError(t, os.WriteFile(redundantPath, []byte("redundant"), 0644))

		// Create a path with double slashes (OS-dependent behavior)
		messyPath := allowedDir + string(filepath.Separator) + string(filepath.Separator) + "redundant.txt"

		content, err := readVaultCredentialFileSecurely(messyPath, allowedDirs)
		require.NoError(t, err)
		require.Equal(t, []byte("redundant"), content)
	})

	t.Run("handles path with dot (current directory) references", func(t *testing.T) {
		dotFile := filepath.Join(allowedDir, "dot.txt")
		require.NoError(t, os.WriteFile(dotFile, []byte("dot-content"), 0644))

		// Path like /var/run/secrets/vault/./file.txt
		dotPath := filepath.Join(allowedDir, ".", "dot.txt")

		content, err := readVaultCredentialFileSecurely(dotPath, allowedDirs)
		require.NoError(t, err)
		require.Equal(t, []byte("dot-content"), content)
	})

	t.Run("rejects directory instead of file", func(t *testing.T) {
		subDir := filepath.Join(allowedDir, "subdir")
		require.NoError(t, os.MkdirAll(subDir, 0755))

		_, err := readVaultCredentialFileSecurely(subDir, allowedDirs)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to read credential file")
	})

	t.Run("handles empty file", func(t *testing.T) {
		emptyFile := filepath.Join(allowedDir, "empty.txt")
		require.NoError(t, os.WriteFile(emptyFile, []byte(""), 0644))

		content, err := readVaultCredentialFileSecurely(emptyFile, allowedDirs)
		require.NoError(t, err)
		require.Equal(t, []byte(""), content)
		require.Len(t, content, 0)
	})

	t.Run("handles file with only whitespace", func(t *testing.T) {
		whitespaceFile := filepath.Join(allowedDir, "whitespace.txt")
		require.NoError(t, os.WriteFile(whitespaceFile, []byte("   \n\t  \n"), 0644))

		content, err := readVaultCredentialFileSecurely(whitespaceFile, allowedDirs)
		require.NoError(t, err)
		require.Equal(t, []byte("   \n\t  \n"), content)
	})

	t.Run("handles permission denied error", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("Skipping permission test when running as root")
		}

		noPermFile := filepath.Join(allowedDir, "noperm.txt")
		require.NoError(t, os.WriteFile(noPermFile, []byte("secret"), 0000))
		defer os.Chmod(noPermFile, 0644) // cleanup

		_, err := readVaultCredentialFileSecurely(noPermFile, allowedDirs)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to read credential file")
	})

	t.Run("handles larger file (1MB)", func(t *testing.T) {
		largeFile := filepath.Join(allowedDir, "large.txt")
		largeContent := make([]byte, 1024*1024) // 1MB
		for i := range largeContent {
			largeContent[i] = byte(i % 256)
		}
		require.NoError(t, os.WriteFile(largeFile, largeContent, 0644))

		content, err := readVaultCredentialFileSecurely(largeFile, allowedDirs)
		require.NoError(t, err)
		require.Equal(t, largeContent, content)
		require.Len(t, content, 1024*1024)
	})

	t.Run("rejects path that starts with allowed dir but escapes", func(t *testing.T) {
		// Attack: /var/run/secrets/vault/../../../etc/passwd
		escapePath := filepath.Join(allowedDir, "..", "..", "..", "etc", "passwd")
		_, err := readVaultCredentialFileSecurely(escapePath, allowedDirs)
		require.Error(t, err)
		require.Contains(t, err.Error(), "credential file must be within allowed directories")
	})

	t.Run("handles file with unicode characters in name", func(t *testing.T) {
		unicodeFile := filepath.Join(allowedDir, "файл-🔐.txt")
		require.NoError(t, os.WriteFile(unicodeFile, []byte("unicode"), 0644))

		content, err := readVaultCredentialFileSecurely(unicodeFile, allowedDirs)
		require.NoError(t, err)
		require.Equal(t, []byte("unicode"), content)
	})

	t.Run("handles file with spaces in name", func(t *testing.T) {
		spaceFile := filepath.Join(allowedDir, "file with spaces.txt")
		require.NoError(t, os.WriteFile(spaceFile, []byte("spaces"), 0644))

		content, err := readVaultCredentialFileSecurely(spaceFile, allowedDirs)
		require.NoError(t, err)
		require.Equal(t, []byte("spaces"), content)
	})

	t.Run("handles relative path when current directory is allowed dir", func(t *testing.T) {
		// Save current directory
		originalWd, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(originalWd)

		// Change to allowed directory
		require.NoError(t, os.Chdir(allowedDir))

		relFile := "relative.txt"
		require.NoError(t, os.WriteFile(relFile, []byte("relative"), 0644))

		// This should fail because we expect absolute paths
		_, err = readVaultCredentialFileSecurely(relFile, allowedDirs)
		require.Error(t, err)
	})

	t.Run("first matching allowed directory wins", func(t *testing.T) {
		// Test that with multiple allowed dirs, first match is used
		firstDir := filepath.Join(tmpDir, "first")
		secondDir := filepath.Join(tmpDir, "second")

		require.NoError(t, os.MkdirAll(firstDir, 0755))
		require.NoError(t, os.MkdirAll(secondDir, 0755))

		testFile := filepath.Join(firstDir, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("first"), 0644))

		multiDirs := []string{firstDir, secondDir}
		content, err := readVaultCredentialFileSecurely(testFile, multiDirs)
		require.NoError(t, err)
		require.Equal(t, []byte("first"), content)
	})

	t.Run("nil allowed directories list", func(t *testing.T) {
		_, err := readVaultCredentialFileSecurely(allowedFile, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "credential file must be within allowed directories")
	})

	t.Run("handles chain of symlinks within allowed directory", func(t *testing.T) {
		// Create a chain using relative paths: symlink1 -> symlink2 -> actual_file
		actualFile := filepath.Join(allowedDir, "actual.txt")
		symlink2 := filepath.Join(allowedDir, "symlink2.txt")
		symlink1 := filepath.Join(allowedDir, "symlink1.txt")

		require.NoError(t, os.WriteFile(actualFile, []byte("chained"), 0644))
		// Use relative symlinks (like Kubernetes secret rotation)
		require.NoError(t, os.Symlink("actual.txt", symlink2))
		require.NoError(t, os.Symlink("symlink2.txt", symlink1))

		content, err := readVaultCredentialFileSecurely(symlink1, allowedDirs)
		require.NoError(t, err)
		require.Equal(t, []byte("chained"), content)
	})

	t.Run("rejects chain of symlinks that escape allowed directory", func(t *testing.T) {
		// Create: symlink1 (in allowed) -> symlink2 (in allowed) -> actual_file (in disallowed)
		symlink2 := filepath.Join(allowedDir, "symlink2-bad.txt")
		symlink1 := filepath.Join(allowedDir, "symlink1-bad.txt")

		require.NoError(t, os.Symlink(disallowedFile, symlink2))
		require.NoError(t, os.Symlink("symlink2-bad.txt", symlink1))

		_, err := readVaultCredentialFileSecurely(symlink1, allowedDirs)
		require.Error(t, err)
		// os.OpenRoot() prevents symlink escape at read time
		require.Contains(t, err.Error(), "failed to read credential file")
	})

	t.Run("handles broken symlink gracefully", func(t *testing.T) {
		// Create a symlink to a non-existent file
		brokenSymlink := filepath.Join(allowedDir, "broken-symlink.txt")
		nonExistent := filepath.Join(allowedDir, "does-not-exist.txt")

		require.NoError(t, os.Symlink(nonExistent, brokenSymlink))

		// This should fail because the file doesn't exist, not because of path validation
		_, err := readVaultCredentialFileSecurely(brokenSymlink, allowedDirs)
		require.Error(t, err)
		// Should fail at read stage, not validation stage
		require.Contains(t, err.Error(), "failed to read credential file")
	})

	t.Run("handles symlink with relative path target within allowed dir", func(t *testing.T) {
		// Create a symlink using relative path
		targetFile := filepath.Join(allowedDir, "target-rel.txt")
		symlinkFile := filepath.Join(allowedDir, "symlink-rel.txt")

		require.NoError(t, os.WriteFile(targetFile, []byte("relative-target"), 0644))

		// Create symlink with relative path (not absolute)
		originalWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(allowedDir))
		require.NoError(t, os.Symlink("target-rel.txt", "symlink-rel.txt"))
		os.Chdir(originalWd)

		content, err := readVaultCredentialFileSecurely(symlinkFile, allowedDirs)
		require.NoError(t, err)
		require.Equal(t, []byte("relative-target"), content)
	})

	t.Run("rejects symlink using relative path to escape", func(t *testing.T) {
		// Create a symlink in allowed dir using relative path to escape
		escapeSymlink := filepath.Join(allowedDir, "escape-symlink.txt")

		// Symlink to ../disallowed/credential.txt - os.OpenRoot() should block this
		require.NoError(t, os.Symlink("../disallowed/credential.txt", escapeSymlink))

		_, err := readVaultCredentialFileSecurely(escapeSymlink, allowedDirs)
		require.Error(t, err)
		// os.OpenRoot() blocks directory traversal via symlinks at read time
		require.Contains(t, err.Error(), "failed to read credential file")
	})

	t.Run("handles symlink to file in subdirectory of allowed dir", func(t *testing.T) {
		subDir := filepath.Join(allowedDir, "subdir")
		require.NoError(t, os.MkdirAll(subDir, 0755))

		targetFile := filepath.Join(subDir, "target-in-sub.txt")
		symlinkFile := filepath.Join(allowedDir, "symlink-to-sub.txt")

		require.NoError(t, os.WriteFile(targetFile, []byte("in-subdir"), 0644))
		// Use relative symlink path
		require.NoError(t, os.Symlink("subdir/target-in-sub.txt", symlinkFile))

		content, err := readVaultCredentialFileSecurely(symlinkFile, allowedDirs)
		require.NoError(t, err)
		require.Equal(t, []byte("in-subdir"), content)
	})

	t.Run("os.OpenRoot blocks symlinks that escape allowed directory", func(t *testing.T) {
		// This test validates the NEW security model: os.OpenRoot() provides OS-level
		// enforcement that prevents symlinks from escaping the rooted filesystem.
		// The symlink itself is validated to be in the allowed directory, but when
		// os.OpenRoot() attempts to follow it to a disallowed location, it fails.
		symlinkInAllowed := filepath.Join(allowedDir, "looks-safe.txt")

		require.NoError(t, os.Symlink(disallowedFile, symlinkInAllowed))

		// The symlink IS in the allowed directory (passes initial validation),
		// but os.OpenRoot() prevents it from being followed to the disallowed location
		_, err := readVaultCredentialFileSecurely(symlinkInAllowed, allowedDirs)
		require.Error(t, err)
		// Error occurs at read time when os.OpenRoot() blocks the escape
		require.Contains(t, err.Error(), "failed to read credential file")
	})
}
