package ca

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-secure-stdlib/awsutil"
	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/api/auth/gcp"

	"github.com/hashicorp/consul/agent/structs"
)

// VaultAuthenticator defines the interface for logging into Vault using an auth method.
type VaultAuthenticator interface {
	// Login to Vault and return a Vault token.
	Login(ctx context.Context, client *api.Client) (*api.Secret, error)
}

// LoginDataGenerator is used to generate the login data for a Vault login API request.
type LoginDataGenerator interface {
	// GenerateLoginData creates the login data from the given authMethod configuration.
	GenerateLoginData(authMethod *structs.VaultAuthMethod) (map[string]interface{}, error)
}

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
	// needed for the API request.
	LoginDataGen LoginDataGenerator
}

// NewVaultAPIAuthClient creates a VaultAuthClient that uses the Vault API to perform a login.
func NewVaultAPIAuthClient(authMethod *structs.VaultAuthMethod, loginPath string) *VaultAuthClient {
	if loginPath == "" {
		mount := authMethod.Type
		if authMethod.MountPath != "" {
			mount = authMethod.MountPath
		}
		loginPath = fmt.Sprintf("auth/%s/login", mount)
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
		loginData, err = c.LoginDataGen.GenerateLoginData(c.AuthMethod)
		if err != nil {
			return nil, fmt.Errorf("aws auth failed to generate login data: %w", err)
		}
	}

	if ctx == nil {
		ctx = context.Background()
	}
	return client.Logical().WriteWithContext(ctx, c.LoginPath, loginData)
}

var _ VaultAuthenticator = (*gcp.GCPAuth)(nil)

// NewGCPAuthClient returns a VaultAuthenticator that can log into Vault using the GCP auth method.
func NewGCPAuthClient(authMethod *structs.VaultAuthMethod) (VaultAuthenticator, error) {
	// Check if the configuration already contains a JWT auth token. If so we want to
	// perform a direct request to the login API with the config that is provided.
	// This supports the  Vault CA config in a backwards compatible way so that we don't
	// break existing configurations.
	if containsVaultLoginParams(authMethod, "jwt") {
		return NewVaultAPIAuthClient(authMethod, ""), nil
	}

	// Create a GCPAuth client from the CA config
	params, err := toMapStringString(authMethod.Params)
	if err != nil {
		return nil, fmt.Errorf("misconfiguration of GCP auth parameters: %w", err)
	}

	opts := []gcp.LoginOption{gcp.WithMountPath(authMethod.MountPath)}

	// Handle the login type explicitly.
	switch params["type"] {
	case "gce":
		opts = append(opts, gcp.WithGCEAuth())
	default:
		opts = append(opts, gcp.WithIAMAuth(params["service_account_email"]))
	}

	auth, err := gcp.NewGCPAuth(params["role"], opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create a new Vault GCP auth client: %w", err)
	}
	return auth, nil
}

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
	if containsVaultLoginParams(authMethod, keys...) {
		return authClient
	}

	// None of the known config is present so use the login data generator to transform
	// the auth method config params into the login data request needed to authenticate
	// via AWS.
	authClient.LoginDataGen = &AWSLoginDataGenerator{}
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
	credsConfig, headerValue, err := NewCredentialsConfig(authMethod.Params)
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
	return loginData, nil
}

// NewCredentialsConfig creates an awsutil.CredentialsConfig from the given set of parameters.
// It is mostly copied from the awsutil package because that package does not currently provide
// an easy way to configure all the available options.
func NewCredentialsConfig(config map[string]interface{}) (*awsutil.CredentialsConfig, string, error) {
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

// containsVaultLoginParams indicates if the provided auth method contains the
// config parameters needed to call the auth/<method>/login API directly.
// It compares the keys in the authMethod.Params struct to the provided slice of
// keys and if any of the keys match it returns true.
func containsVaultLoginParams(authMethod *structs.VaultAuthMethod, keys ...string) bool {
	for _, key := range keys {
		if _, exists := authMethod.Params[key]; exists {
			return true
		}
	}
	return false
}
