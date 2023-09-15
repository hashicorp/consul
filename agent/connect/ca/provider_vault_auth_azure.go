// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package ca

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/vault/sdk/helper/jsonutil"
)

func NewAzureAuthClient(authMethod *structs.VaultAuthMethod) (*VaultAuthClient, error) {
	params := authMethod.Params
	authClient := NewVaultAPIAuthClient(authMethod, "")
	// check for login data already in params (for backwards compability)
	legacyKeys := []string{
		"vm_name", "vmss_name", "resource_group_name", "subscription_id", "jwt",
	}
	if legacyCheck(params, legacyKeys...) {
		return authClient, nil
	}

	role, ok := params["role"].(string)
	if !ok || strings.TrimSpace(role) == "" {
		return nil, fmt.Errorf("missing 'role' value")
	}
	resource, ok := params["resource"].(string)
	if !ok || strings.TrimSpace(resource) == "" {
		return nil, fmt.Errorf("missing 'resource' value")
	}

	authClient.LoginDataGen = AzureLoginDataGen
	return authClient, nil
}

var ( // use variables so we can change these in tests
	instanceEndpoint = "http://169.254.169.254/metadata/instance"
	identityEndpoint = "http://169.254.169.254/metadata/identity/oauth2/token"
	// minimum version 2018-02-01 needed for identity metadata
	apiVersion = "2018-02-01"
)

type instanceData struct {
	Compute Compute
}
type Compute struct {
	Name              string
	ResourceGroupName string
	SubscriptionID    string
	VMScaleSetName    string
}
type identityData struct {
	AccessToken string `json:"access_token"`
}

func AzureLoginDataGen(authMethod *structs.VaultAuthMethod) (map[string]any, error) {
	params := authMethod.Params
	role := params["role"].(string)
	metaConf := map[string]string{
		"role":     role,
		"resource": params["resource"].(string),
	}
	if objectID, ok := params["object_id"].(string); ok {
		metaConf["object_id"] = objectID
	}
	if clientID, ok := params["client_id"].(string); ok {
		metaConf["client_id"] = clientID
	}

	// Fetch instance data
	var instance instanceData
	body, err := getMetadataInfo(instanceEndpoint, nil)
	if err != nil {
		return nil, err
	}
	err = jsonutil.DecodeJSON(body, &instance)
	if err != nil {
		return nil, fmt.Errorf("error parsing instance metadata response: %w", err)
	}

	// Fetch JWT
	var identity identityData
	body, err = getMetadataInfo(identityEndpoint, metaConf)
	if err != nil {
		return nil, err
	}
	err = jsonutil.DecodeJSON(body, &identity)
	if err != nil {
		return nil, fmt.Errorf("error parsing instance metadata response: %w", err)
	}

	data := map[string]interface{}{
		"role":                role,
		"vm_name":             instance.Compute.Name,
		"vmss_name":           instance.Compute.VMScaleSetName,
		"resource_group_name": instance.Compute.ResourceGroupName,
		"subscription_id":     instance.Compute.SubscriptionID,
		"jwt":                 identity.AccessToken,
	}

	return data, nil
}

func getMetadataInfo(endpoint string, query map[string]string) ([]byte, error) {
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("api-version", apiVersion)
	for k, v := range query {
		q.Add(k, v)
	}
	req.URL.RawQuery = q.Encode()
	req.Header.Set("Metadata", "true")
	req.Header.Set("User-Agent", "Consul")

	client := cleanhttp.DefaultClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching metadata from %s: %w", endpoint, err)
	}

	if resp == nil {
		return nil, fmt.Errorf("empty response fetching metadata from %s", endpoint)
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading metadata from %s: %w", endpoint, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error response in metadata from %s: %s", endpoint, body)
	}

	return body, nil
}
