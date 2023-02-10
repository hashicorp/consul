package ca

import (
	"fmt"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/auth/credentials/providers"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/vault-plugin-auth-alicloud/tools"
)

func NewAliCloudAuthClient(authMethod *structs.VaultAuthMethod) (*VaultAuthClient, error) {
	if _, ok := authMethod.Params["role"].(string); !ok {
		return nil, fmt.Errorf("role is required for AliCloud login")
	}
	if _, ok := authMethod.Params["region"].(string); !ok {
		return nil, fmt.Errorf("region is required for AliCloud login")
	}
	client := NewVaultAPIAuthClient(authMethod, "")
	client.LoginDataGen = AliLoginDataGen
	return client, nil
}

func AliLoginDataGen(authMethod *structs.VaultAuthMethod) (map[string]any, error) {
	// validity of these params is checked above in New..
	role := authMethod.Params["role"].(string)
	region := authMethod.Params["region"].(string)
	// Credentials can be provided either explicitly via env vars,
	// or we will try to derive them from instance metadata.
	credentialChain := []providers.Provider{
		providers.NewEnvCredentialProvider(),
		providers.NewInstanceMetadataProvider(),
	}
	creds, err := providers.NewChainProvider(credentialChain).Retrieve()
	if err != nil {
		return nil, err
	}

	loginData, err := tools.GenerateLoginData(role, creds, region)
	if err != nil {
		return nil, err
	}

	return loginData, nil
}
