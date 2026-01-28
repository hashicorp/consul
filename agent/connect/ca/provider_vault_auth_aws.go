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
	awsutilv2 "github.com/hashicorp/go-secure-stdlib/awsutil/v2"

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

	// Use consul-awsauth's GenerateLoginData which works with AWS SDK v2
	loginData, err := iamauth.GenerateLoginData(&iamauth.LoginInput{
		Creds:               awsConfig.Credentials,
		IncludeIAMEntity:    false, // For Vault auth, we don't include IAM entity details
		STSRegion:           awsConfig.Region,
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

// newAWSCredentialsConfig creates an awsutilv2.CredentialsConfig from the given set of parameters.
func newAWSCredentialsConfig(config map[string]interface{}) (*awsutilv2.CredentialsConfig, string, error) {
	params, err := toMapStringString(config)
	if err != nil {
		return nil, "", fmt.Errorf("misconfiguration of AWS auth parameters: %w", err)
	}

	// Build options for awsutilv2.NewCredentialsConfig
	var opts []awsutilv2.Option

	// Add basic credentials if provided
	if accessKey := params["access_key"]; accessKey != "" {
		opts = append(opts, awsutilv2.WithAccessKey(accessKey))
	}
	if secretKey := params["secret_key"]; secretKey != "" {
		opts = append(opts, awsutilv2.WithSecretKey(secretKey))
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
	opts = append(opts, awsutilv2.WithRegion(region))

	// Add optional parameters
	if roleArn := params["role_arn"]; roleArn != "" {
		opts = append(opts, awsutilv2.WithRoleArn(roleArn))
	}
	if roleSessionName := params["role_session_name"]; roleSessionName != "" {
		opts = append(opts, awsutilv2.WithRoleSessionName(roleSessionName))
	}
	if roleExternalId := params["role_external_id"]; roleExternalId != "" {
		opts = append(opts, awsutilv2.WithRoleExternalId(roleExternalId))
	}
	if webIdentityTokenFile := params["web_identity_token_file"]; webIdentityTokenFile != "" {
		opts = append(opts, awsutilv2.WithWebIdentityTokenFile(webIdentityTokenFile))
	}
	if webIdentityToken := params["web_identity_token"]; webIdentityToken != "" {
		opts = append(opts, awsutilv2.WithWebIdentityToken(webIdentityToken))
	}

	// Add HTTP client
	opts = append(opts, awsutilv2.WithHttpClient(cleanhttp.DefaultClient()))

	// Add max retries if specified
	if maxRetriesStr := params["max_retries"]; maxRetriesStr != "" {
		if maxRetries, err := strconv.Atoi(maxRetriesStr); err == nil {
			opts = append(opts, awsutilv2.WithMaxRetries(&maxRetries))
		}
	}

	// Add endpoint resolvers if specified
	// Note: IAM and STS endpoints in v2 require EndpointResolverV2 interfaces
	// For now, we'll skip custom endpoints as they require more complex setup
	// If needed in the future, we can create custom endpoint resolvers

	// Create the credentials config
	c, err := awsutilv2.NewCredentialsConfig(opts...)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create AWS credentials config: %w", err)
	}

	return c, params["header_value"], nil
}
