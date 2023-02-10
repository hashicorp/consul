//  go:build ignore

package ca

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	cf "github.com/hashicorp/vault-plugin-auth-cf"
	"github.com/hashicorp/vault-plugin-auth-cf/signatures"
)

// Cloud Foundry (CF) Auth needs to read from some
func NewCFAuthClient(authMethod *structs.VaultAuthMethod) (*VaultAuthClient, error) {
	// check params in New method..
	if _, ok := authMethod.Params["role"].(string); !ok {
		return nil, errors.New("missing 'role' value")
	}
	if reqEnv := os.Getenv(cf.EnvVarInstanceCertificate); reqEnv == "" {
		return nil, fmt.Errorf("missing environment value: %q",
			cf.EnvVarInstanceCertificate)
	}
	if reqEnv := os.Getenv(cf.EnvVarInstanceKey); reqEnv == "" {
		return nil, fmt.Errorf("missing environment value: %q", cf.EnvVarInstanceKey)
	}

	authClient := NewVaultAPIAuthClient(authMethod, "")
	authClient.LoginDataGen = CFLoginDataGen
	return authClient, nil
}

func CFLoginDataGen(authMethod *structs.VaultAuthMethod) (map[string]interface{}, error) {
	role := authMethod.Params["role"].(string)

	// keep environmnent variable checks in LoginDataGen to facilitate testing
	pathToClientCert := os.Getenv(cf.EnvVarInstanceCertificate)

	certBytes, err := os.ReadFile(pathToClientCert)
	if err != nil {
		return nil, err
	}
	signingTime := time.Now().UTC()
	signatureData := &signatures.SignatureData{
		SigningTime:            signingTime,
		Role:                   role,
		CFInstanceCertContents: string(certBytes),
	}
	pathToClientKey := os.Getenv(cf.EnvVarInstanceKey)
	signature, err := signatures.Sign(pathToClientKey, signatureData)
	if err != nil {
		return nil, err
	}
	data := map[string]interface{}{
		"role":             role,
		"cf_instance_cert": string(certBytes),
		"signing_time":     signingTime.Format(signatures.TimeFormat),
		"signature":        signature,
	}
	return data, nil
}
