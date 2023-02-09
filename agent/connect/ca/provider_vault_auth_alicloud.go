package ca

import (
	"fmt"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/auth/credentials/providers"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/vault-plugin-auth-alicloud/tools"
)

func NewAliCloudAuthClient(authMethod *structs.VaultAuthMethod) *VaultAuthClient {
	client := NewVaultAPIAuthClient(authMethod, "")
	client.LoginDataGen = AliLoginDataGen
	return client
}

func AliLoginDataGen(authMethod *structs.VaultAuthMethod) (map[string]any, error) {
	role, ok := authMethod.Params["role"].(string)
	if !ok {
		return nil, fmt.Errorf("role is required for AliCloud login")
	}
	region, ok := authMethod.Params["region"].(string)
	if !ok {
		return nil, fmt.Errorf("region is required for AliCloud login")
	}
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
