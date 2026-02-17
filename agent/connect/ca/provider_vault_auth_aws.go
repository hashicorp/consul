// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package ca

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	iamauth "github.com/hashicorp/consul-awsauth"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-secure-stdlib/awsutil/v2"

	"github.com/hashicorp/consul/agent/structs"
)

// NewAWSAuthClient returns a VaultAuthClient that can log into Vault using the AWS auth method.
func NewAWSAuthClient(authMethod *structs.VaultAuthMethod) *VaultAuthClient {
	authClient := NewVaultAPIAuthClient(authMethod, "")

	// Inspect the config params to see if they are already in the format required for
	// the auth/aws/login API call. If so, we want to call the login API with the params
	// exactly as they are configured. This supports the  Vault CA config in a backwards
	// compatible way so that we don't break existing configurations.
	keys := []string{
		"identity",                // EC2 identity
		"pkcs7",                   // EC2 PKCS7
		"iam_http_request_method", // IAM
	}
	if legacyCheck(authMethod.Params, keys...) {
		return authClient
	}

	// None of the known config is present so use the login data generator to transform
	// the auth method config params into the login data request needed to authenticate
	// via AWS.
	lgd := &AWSLoginDataGenerator{}
	authClient.LoginDataGen = lgd.GenerateLoginData
	return authClient
}

// AWSLoginDataGenerator is a LoginDataGenerator for AWS authentication.
type AWSLoginDataGenerator struct {
	// awsConfig supports explicitly setting the AWS config to use for authenticating with AWS.
	// It is un-exported and intended for testing purposes only.
	awsConfig *aws.Config
}

// GenerateLoginData derives the login data for the Vault AWS auth login request from the CA
// provider auth method config.
func (g *AWSLoginDataGenerator) GenerateLoginData(authMethod *structs.VaultAuthMethod) (map[string]interface{}, error) {
	ctx := context.Background()

	// Create the credential configuration from the parameters.
	credsConfig, headerValue, err := newAWSCredentialsConfig(authMethod.Params)
	if err != nil {
		return nil, fmt.Errorf("aws auth failed to create credential configuration: %w", err)
	}

	// Use the provided AWS config if present, otherwise generate one
	awsConfig := g.awsConfig
	if awsConfig == nil {
		// Generate the AWS config with credentials
		awsConfig, err = credsConfig.GenerateCredentialChain(ctx)
		if err != nil {
			return nil, fmt.Errorf("aws auth failed to generate credential chain: %w", err)
		}
	}

	// Extract STS endpoint from params if provided
	// This is needed because consul-awsauth creates its own STS client
	stsEndpoint := ""
	if ep, ok := authMethod.Params["sts_endpoint"].(string); ok {
		stsEndpoint = ep
	}

	// Extract IAM endpoint from params if provided
	// This is needed because consul-awsauth creates its own IAM client internally
	iamEndpoint := ""
	if ep, ok := authMethod.Params["iam_endpoint"].(string); ok {
		iamEndpoint = ep
	}

	// Use consul-awsauth's GenerateLoginData which works with AWS SDK v2
	// Note: IncludeIAMEntity is set to false for Vault authentication because:
	// 1. Vault's AWS auth method doesn't use the entity headers that consul-awsauth requires
	// 2. The entity headers (GetEntityMethodHeader, etc.) are specific to Consul's auth method
	// 3. Vault validates AWS identity differently and doesn't need IAM entity details in the request
	loginData, err := iamauth.GenerateLoginData(&iamauth.LoginInput{
		Creds:               awsConfig.Credentials,
		IncludeIAMEntity:    false, // Always false for Vault auth (entity headers not applicable)
		STSRegion:           awsConfig.Region,
		STSEndpoint:         stsEndpoint, // Pass custom STS endpoint if specified
		IAMEndpoint:         iamEndpoint, // Pass custom IAM endpoint if specified
		ServerIDHeaderValue: headerValue,
		Logger:              hclog.NewNullLogger(),
	})
	if err != nil {
		return nil, fmt.Errorf("aws auth failed to generate login data: %w", err)
	}

	// If a Vault role name is specified, we need to manually add this
	role, ok := authMethod.Params["role"]
	if ok {
		loginData["role"] = role
	}

	return loginData, nil
}

// newAWSCredentialsConfig creates an awsutil.CredentialsConfig from the given set of parameters.
func newAWSCredentialsConfig(config map[string]interface{}) (*awsutil.CredentialsConfig, string, error) {
	// Filter out non-credential parameters before passing to awsutil
	// These parameters (endpoints, role) are handled separately in the auth flow
	filteredConfig := make(map[string]interface{})
	for k, v := range config {
		// Skip parameters that aren't credentials-related
		if k == "sts_endpoint" || k == "iam_endpoint" || k == "role" {
			continue
		}
		filteredConfig[k] = v
	}

	params, err := toMapStringString(filteredConfig)
	if err != nil {
		return nil, "", fmt.Errorf("misconfiguration of AWS auth parameters: %w", err)
	}

	// Build options for awsutil.NewCredentialsConfig
	var opts []awsutil.Option

	// Add basic credentials if provided
	if accessKey := params["access_key"]; accessKey != "" {
		opts = append(opts, awsutil.WithAccessKey(accessKey))
	}
	if secretKey := params["secret_key"]; secretKey != "" {
		opts = append(opts, awsutil.WithSecretKey(secretKey))
	}

	// Add region
	region := params["region"]
	if region == "" {
		region = os.Getenv("AWS_REGION")
		if region == "" {
			region = os.Getenv("AWS_DEFAULT_REGION")
			if region == "" {
				region = "us-east-1"
			}
		}
	}
	opts = append(opts, awsutil.WithRegion(region))

	// Add optional parameters
	if roleArn := params["role_arn"]; roleArn != "" {
		opts = append(opts, awsutil.WithRoleArn(roleArn))
	}
	if roleSessionName := params["role_session_name"]; roleSessionName != "" {
		opts = append(opts, awsutil.WithRoleSessionName(roleSessionName))
	}
	if roleExternalId := params["role_external_id"]; roleExternalId != "" {
		opts = append(opts, awsutil.WithRoleExternalId(roleExternalId))
	}
	if webIdentityTokenFile := params["web_identity_token_file"]; webIdentityTokenFile != "" {
		opts = append(opts, awsutil.WithWebIdentityTokenFile(webIdentityTokenFile))
	}
	if webIdentityToken := params["web_identity_token"]; webIdentityToken != "" {
		opts = append(opts, awsutil.WithWebIdentityToken(webIdentityToken))
	}

	// Add HTTP client
	opts = append(opts, awsutil.WithHttpClient(cleanhttp.DefaultClient()))

	// Add max retries if specified
	if maxRetriesStr := params["max_retries"]; maxRetriesStr != "" {
		if maxRetries, err := strconv.Atoi(maxRetriesStr); err == nil {
			opts = append(opts, awsutil.WithMaxRetries(&maxRetries))
		}
	}

	// Note: IAM and STS endpoint configuration is NOT added to awsutil config here
	// because consul-awsauth creates its own STS/IAM clients and doesn't use the
	// awsutil-provided config. Instead, we extract the endpoint strings from params
	// and pass them directly to consul-awsauth.GenerateLoginData() (see lines 74-86).

	// Create the credentials config
	c, err := awsutil.NewCredentialsConfig(opts...)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create AWS credentials config: %w", err)
	}

	// Set session token directly on the config struct as there is no WithSessionToken option in awsutil
	if sessionToken := params["session_token"]; sessionToken != "" {
		c.SessionToken = sessionToken
	}

	// Set filename (custom credential file path) if provided
	// This allows using a custom AWS credentials file instead of the default ~/.aws/credentials
	if filename := params["filename"]; filename != "" {
		c.Filename = filename
	}

	// Set profile (AWS profile selection) if provided
	// This allows selecting a specific profile from the AWS credentials/config files
	if profile := params["profile"]; profile != "" {
		c.Profile = profile
	}

	return c, params["header_value"], nil
}
