package ca

import (
	"context"
	"fmt"

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
