package ca

import (
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/consul/agent/structs"
)

func NewJwtAuthClient(authMethod *structs.VaultAuthMethod) (*VaultAuthClient, error) {
	params := authMethod.Params

	role, ok := params["role"].(string)
	if !ok || strings.TrimSpace(role) == "" {
		return nil, fmt.Errorf("missing 'role' value")
	}

	// The path is required for the auto-auth config, but this auth provider
	// seems to be used for jwt based auth by directly passing the jwt token.
	// So we only require the token file path if the token string isn't
	// present.
	needTokenPath := true
	// support legacy setup that allows directly passing the `jwt`
	if _, ok := hasJWT(params); ok {
		needTokenPath = false
	}
	if needTokenPath {
		tokenPath, ok := params["path"].(string)
		if !ok || strings.TrimSpace(tokenPath) == "" {
			return nil, fmt.Errorf("missing 'path' value")
		}
	}

	authClient := NewVaultAPIAuthClient(authMethod, "")
	authClient.LoginDataGen = JwtLoginDataGen
	return authClient, nil
}

func JwtLoginDataGen(authMethod *structs.VaultAuthMethod) (map[string]any, error) {
	params := authMethod.Params
	role := params["role"].(string)

	// support legacy setup that allows directly passing the `jwt`
	if jwt, ok := hasJWT(params); ok {
		return map[string]any{
			"role": role,
			"jwt":  jwt,
		}, nil
	}

	tokenPath := params["path"].(string)

	rawToken, err := os.ReadFile(tokenPath)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"role": role,
		"jwt":  strings.TrimSpace(string(rawToken)),
	}, nil
}

// Note the `jwt` can be passed directly in the authMethod as the it's Params
// is a freeform map in the config where they could hardcode it.
// See comment on configureVaultAuthMethod (in ./provider_vault.go) for more.
func hasJWT(params map[string]any) (string, bool) {
	if jwt, ok := params["jwt"].(string); ok && strings.TrimSpace(jwt) != "" {
		return jwt, true
	}
	return "", false
}
