package ca

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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
		expErr error
	}{
		"valid login data": {},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			ldg := &AWSLoginDataGenerator{credentials: credentials.AnonymousCredentials}
			loginData, err := ldg.GenerateLoginData(&structs.VaultAuthMethod{})
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
