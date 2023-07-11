// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package ca

import (
	"fmt"
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-secure-stdlib/awsutil"

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
	// credentials supports explicitly setting the AWS credentials to use for authenticating with AWS.
	// It is un-exported and intended for testing purposes only.
	credentials *credentials.Credentials
}

// GenerateLoginData derives the login data for the Vault AWS auth login request from the CA
// provider auth method config.
func (g *AWSLoginDataGenerator) GenerateLoginData(authMethod *structs.VaultAuthMethod) (map[string]interface{}, error) {
	// Create the credential configuration from the parameters.
	credsConfig, headerValue, err := newAWSCredentialsConfig(authMethod.Params)
	if err != nil {
		return nil, fmt.Errorf("aws auth failed to create credential configuration: %w", err)
	}

	// Use the provided credentials if they are present.
	creds := g.credentials
	if creds == nil {
		// Generate the credential chain
		creds, err = credsConfig.GenerateCredentialChain()
		if err != nil {
			return nil, fmt.Errorf("aws auth failed to generate credential chain: %w", err)
		}
	}

	// Derive the login data
	loginData, err := awsutil.GenerateLoginData(creds, headerValue, credsConfig.Region, hclog.NewNullLogger())
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
// It is mostly copied from the awsutil package because that package does not currently provide
// an easy way to configure all the available options.
func newAWSCredentialsConfig(config map[string]interface{}) (*awsutil.CredentialsConfig, string, error) {
	params, err := toMapStringString(config)
	if err != nil {
		return nil, "", fmt.Errorf("misconfiguration of AWS auth parameters: %w", err)
	}

	// Populate the AWS credentials configuration. Empty strings will use the defaults.
	c := &awsutil.CredentialsConfig{
		AccessKey:            params["access_key"],
		SecretKey:            params["secret_key"],
		SessionToken:         params["session_token"],
		IAMEndpoint:          params["iam_endpoint"],
		STSEndpoint:          params["sts_endpoint"],
		Region:               params["region"],
		Filename:             params["filename"],
		Profile:              params["profile"],
		RoleARN:              params["role_arn"],
		RoleSessionName:      params["role_session_name"],
		WebIdentityTokenFile: params["web_identity_token_file"],
		HTTPClient:           cleanhttp.DefaultClient(),
	}

	if maxRetries, err := strconv.Atoi(params["max_retries"]); err == nil {
		c.MaxRetries = &maxRetries
	}

	if c.Region == "" {
		c.Region = os.Getenv("AWS_REGION")
		if c.Region == "" {
			c.Region = os.Getenv("AWS_DEFAULT_REGION")
			if c.Region == "" {
				c.Region = "us-east-1"
			}
		}
	}

	return c, params["header_value"], nil
}
