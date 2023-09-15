// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package ca

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-secure-stdlib/awsutil"
	"github.com/hashicorp/vault/api/auth/gcp"
	"github.com/hashicorp/vault/sdk/helper/jsonutil"
	"github.com/stretchr/testify/require"
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
		expCreds  *awsutil.CredentialsConfig
		expErr    error
		expRegion string
	}{
		"valid config": {
			params: map[string]interface{}{
				"access_key":              "access key",
				"secret_key":              "secret key",
				"session_token":           "session token",
				"iam_endpoint":            "iam endpoint",
				"sts_endpoint":            "sts endpoint",
				"region":                  "region",
				"filename":                "filename",
				"profile":                 "profile",
				"role_arn":                "role arn",
				"role_session_name":       "role session name",
				"web_identity_token_file": "web identity token file",
				"header_value":            "header value",
				"max_retries":             "13",
			},
			expCreds: &awsutil.CredentialsConfig{
				AccessKey:            "access key",
				SecretKey:            "secret key",
				SessionToken:         "session token",
				IAMEndpoint:          "iam endpoint",
				STSEndpoint:          "sts endpoint",
				Region:               "region",
				Filename:             "filename",
				Profile:              "profile",
				RoleARN:              "role arn",
				RoleSessionName:      "role session name",
				WebIdentityTokenFile: "web identity token file",
			},
		},
		"default region": {
			params:    map[string]interface{}{},
			expCreds:  &awsutil.CredentialsConfig{},
			expRegion: "us-east-1",
		},
		"env AWS_REGION": {
			params:    map[string]interface{}{},
			envVars:   map[string]string{"AWS_REGION": "us-west-1"},
			expCreds:  &awsutil.CredentialsConfig{},
			expRegion: "us-west-1",
		},
		"env AWS_DEFAULT_REGION": {
			params:    map[string]interface{}{},
			envVars:   map[string]string{"AWS_DEFAULT_REGION": "us-west-2"},
			expCreds:  &awsutil.CredentialsConfig{},
			expRegion: "us-west-2",
		},
		"both AWS_REGION and AWS_DEFAULT_REGION": {
			params: map[string]interface{}{},
			envVars: map[string]string{
				"AWS_REGION":         "us-west-1",
				"AWS_DEFAULT_REGION": "us-west-2",
			},
			expCreds:  &awsutil.CredentialsConfig{},
			expRegion: "us-west-1",
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

			// If a header value was provided in the params then make sure it was returned.
			if val, ok := c.params["header_value"]; ok {
				require.Equal(t, val, headerValue)
			} else {
				require.Empty(t, headerValue)
			}

			if val, ok := c.params["max_retries"]; ok {
				mr, err := strconv.Atoi(val.(string))
				require.NoError(t, err)
				c.expCreds.MaxRetries = &mr
			} else {
				creds.MaxRetries = nil
			}

			require.NotNil(t, creds.HTTPClient)
			creds.HTTPClient = nil

			if c.expRegion != "" {
				c.expCreds.Region = c.expRegion
			}
			require.Equal(t, *c.expCreds, *creds)
		})
	}
}

func TestVaultCAProvider_AWSLoginDataGenerator(t *testing.T) {
	cases := map[string]struct {
		expErr     error
		authMethod structs.VaultAuthMethod
	}{
		"valid login data": {
			authMethod: structs.VaultAuthMethod{},
		},
		"with role": {
			expErr:     nil,
			authMethod: structs.VaultAuthMethod{Type: "aws", MountPath: "", Params: map[string]interface{}{"role": "test-role"}},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			ldg := &AWSLoginDataGenerator{credentials: credentials.AnonymousCredentials}
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
	tokenF, err := os.CreateTemp("", "token-path")
	require.NoError(t, err)
	defer func() { os.Remove(tokenF.Name()) }()
	_, err = tokenF.WriteString("test-token")
	require.NoError(t, err)
	err = tokenF.Close()
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
					"path": tokenF.Name(),
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
	tokenF, err := os.CreateTemp("", "token-path")
	require.NoError(t, err)
	defer func() { os.Remove(tokenF.Name()) }()
	_, err = tokenF.WriteString("test-token")
	require.NoError(t, err)
	err = tokenF.Close()
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
					"token_path": tokenF.Name(),
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

	roleFd, err := os.CreateTemp("", "role")
	require.NoError(t, err)
	_, err = roleFd.WriteString(roleID)
	require.NoError(t, err)
	err = roleFd.Close()
	require.NoError(t, err)

	secretFd, err := os.CreateTemp("", "secret")
	require.NoError(t, err)
	_, err = secretFd.WriteString(secretID)
	require.NoError(t, err)
	err = secretFd.Close()
	require.NoError(t, err)

	roleIdPath := roleFd.Name()
	secretIdPath := secretFd.Name()

	defer func() {
		os.Remove(secretFd.Name())
		os.Remove(roleFd.Name())
	}()

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
