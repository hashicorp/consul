// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package awsauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/stretchr/testify/require"

	iamauth "github.com/hashicorp/consul-awsauth"
	"github.com/hashicorp/consul-awsauth/iamauthtest"
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/consul/authmethod"
	"github.com/hashicorp/consul/agent/structs"
)

func TestNewValidator(t *testing.T) {
	f := iamauthtest.MakeFixture()
	expConfig := &iamauth.Config{
		BoundIAMPrincipalARNs:  []string{f.AssumedRoleARN},
		EnableIAMEntityDetails: true,
		IAMEntityTags:          []string{"tag-1"},
		ServerIDHeaderValue:    "x-some-header",
		MaxRetries:             3,
		IAMEndpoint:            "http://iam-endpoint",
		STSEndpoint:            "http://sts-endpoint",
		AllowedSTSHeaderValues: []string{"header-value"},
		ServerIDHeaderName:     "X-Consul-IAM-ServerID",
		GetEntityMethodHeader:  "X-Consul-IAM-GetEntity-Method",
		GetEntityURLHeader:     "X-Consul-IAM-GetEntity-URL",
		GetEntityHeadersHeader: "X-Consul-IAM-GetEntity-Headers",
		GetEntityBodyHeader:    "X-Consul-IAM-GetEntity-Body",
	}

	type AM = *structs.ACLAuthMethod
	// Create the auth method, with an optional modification function.
	makeMethod := func(modifyFn func(AM)) AM {
		config := map[string]interface{}{
			"BoundIAMPrincipalARNs":  []string{f.AssumedRoleARN},
			"EnableIAMEntityDetails": true,
			"IAMEntityTags":          []string{"tag-1"},
			"ServerIDHeaderValue":    "x-some-header",
			"MaxRetries":             3,
			"IAMEndpoint":            "http://iam-endpoint",
			"STSEndpoint":            "http://sts-endpoint",
			"AllowedSTSHeaderValues": []string{"header-value"},
		}

		m := &structs.ACLAuthMethod{
			Name:        "test-iam",
			Type:        "aws-iam",
			Description: "aws iam auth",
			Config:      config,
		}
		if modifyFn != nil {
			modifyFn(m)
		}
		return m
	}

	cases := map[string]struct {
		ok       bool
		modifyFn func(AM)
	}{
		"success":                  {true, nil},
		"wrong type":               {false, func(m AM) { m.Type = "not-iam" }},
		"extra config":             {false, func(m AM) { m.Config["extraField"] = "123" }},
		"wrong config value type":  {false, func(m AM) { m.Config["MaxRetries"] = []string{"1"} }},
		"missing bound principals": {false, func(m AM) { delete(m.Config, "BoundIAMPrincipalARNs") }},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			v, err := NewValidator(nil, makeMethod(c.modifyFn))
			if c.ok {
				require.NoError(t, err)
				require.NotNil(t, v)
				require.Equal(t, "test-iam", v.name)
				require.NotNil(t, v.auth)
				require.Equal(t, expConfig, v.config)
			} else {
				require.Error(t, err)
				require.Nil(t, v)
			}
		})
	}
}

func TestValidateLogin(t *testing.T) {
	f := iamauthtest.MakeFixture()

	cases := map[string]struct {
		server    *iamauthtest.Server
		token     string
		config    map[string]interface{}
		expVars   map[string]string
		expFields []string
		expError  string
	}{
		"success - role login": {
			server: f.ServerForRole,
			config: map[string]interface{}{
				"BoundIAMPrincipalARNs": []string{f.CanonicalRoleARN},
			},
			expVars: map[string]string{
				"entity_id":   f.EntityID,
				"entity_name": f.RoleName,
				"account_id":  f.AccountID,
			},
			expFields: []string{
				fmt.Sprintf(`entity_id == %q`, f.EntityID),
				fmt.Sprintf(`entity_name == %q`, f.RoleName),
				fmt.Sprintf(`account_id == %q`, f.AccountID),
			},
		},
		"success - user login": {
			server: f.ServerForUser,
			config: map[string]interface{}{
				"BoundIAMPrincipalARNs": []string{f.UserARN},
			},
			expVars: map[string]string{
				"entity_id":   f.EntityID,
				"entity_name": f.UserName,
				"account_id":  f.AccountID,
			},
			expFields: []string{
				fmt.Sprintf(`entity_id == %q`, f.EntityID),
				fmt.Sprintf(`entity_name == %q`, f.UserName),
				fmt.Sprintf(`account_id == %q`, f.AccountID),
			},
		},
		"success - role login with entity details": {
			server: f.ServerForUser,
			config: map[string]interface{}{
				"BoundIAMPrincipalARNs":  []string{f.UserARN},
				"EnableIAMEntityDetails": true,
			},
			expVars: map[string]string{
				"entity_id":   f.EntityID,
				"entity_name": f.UserName,
				"account_id":  f.AccountID,
				"entity_path": f.UserPath,
			},
			expFields: []string{
				fmt.Sprintf(`entity_id == %q`, f.EntityID),
				fmt.Sprintf(`entity_name == %q`, f.UserName),
				fmt.Sprintf(`account_id == %q`, f.AccountID),
				fmt.Sprintf(`entity_path == %q`, f.UserPath),
			},
		},
		"success - user login with entity details": {
			server: f.ServerForUser,
			config: map[string]interface{}{
				"BoundIAMPrincipalARNs":  []string{f.UserARN},
				"EnableIAMEntityDetails": true,
			},
			expVars: map[string]string{
				"entity_id":   f.EntityID,
				"entity_name": f.UserName,
				"account_id":  f.AccountID,
				"entity_path": f.UserPath,
			},
			expFields: []string{
				fmt.Sprintf(`entity_id == %q`, f.EntityID),
				fmt.Sprintf(`entity_name == %q`, f.UserName),
				fmt.Sprintf(`account_id == %q`, f.AccountID),
				fmt.Sprintf(`entity_path == %q`, f.UserPath),
			},
		},
		"invalid token": {
			server: f.ServerForUser,
			config: map[string]interface{}{
				"BoundIAMPrincipalARNs": []string{f.UserARN},
			},
			token:    `invalid`,
			expError: "invalid token",
		},
		"empty json token": {
			server: f.ServerForUser,
			config: map[string]interface{}{
				"BoundIAMPrincipalARNs": []string{f.UserARN},
			},
			token:    `{}`,
			expError: "invalid token",
		},
		"empty json fields in token": {
			server: f.ServerForUser,
			config: map[string]interface{}{
				"BoundIAMPrincipalARNs": []string{f.UserARN},
			},
			token: `{"iam_http_request_method": "",
"iam_request_body": "",
"iam_request_headers": "",
"iam_request_url": ""
}`,
			expError: "invalid token",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			v, _, token := setup(t, c.config, c.server)
			if c.token != "" {
				token = c.token
			}
			id, err := v.ValidateLogin(context.Background(), token)
			if c.expError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), c.expError)
				require.Nil(t, id)
			} else {
				require.NoError(t, err)
				authmethod.RequireIdentityMatch(t, id, c.expVars, c.expFields...)
			}
		})
	}
}

func setup(t *testing.T, config map[string]interface{}, server *iamauthtest.Server) (*Validator, *httptest.Server, string) {
	t.Helper()

	fakeAws := iamauthtest.NewTestServer(t, server)

	config["STSEndpoint"] = fakeAws.URL + "/sts"
	config["IAMEndpoint"] = fakeAws.URL + "/iam"

	method := &structs.ACLAuthMethod{
		Name:   "test-method",
		Type:   "aws-iam",
		Config: config,
	}
	nullLogger := hclog.NewNullLogger()
	v, err := NewValidator(nullLogger, method)
	require.NoError(t, err)

	// Generate the login token
	tokenData, err := iamauth.GenerateLoginData(&iamauth.LoginInput{
		Creds:                  credentials.NewStaticCredentials("fake", "fake", ""),
		IncludeIAMEntity:       v.config.EnableIAMEntityDetails,
		STSEndpoint:            v.config.STSEndpoint,
		STSRegion:              "fake-region",
		Logger:                 nullLogger,
		ServerIDHeaderValue:    v.config.ServerIDHeaderValue,
		ServerIDHeaderName:     v.config.ServerIDHeaderName,
		GetEntityMethodHeader:  v.config.GetEntityMethodHeader,
		GetEntityURLHeader:     v.config.GetEntityURLHeader,
		GetEntityHeadersHeader: v.config.GetEntityHeadersHeader,
		GetEntityBodyHeader:    v.config.GetEntityBodyHeader,
	})
	require.NoError(t, err)

	token, err := json.Marshal(tokenData)
	require.NoError(t, err)
	return v, fakeAws, string(token)
}

func TestNewIdentity(t *testing.T) {
	principals := []string{"arn:aws:sts::1234567890:assumed-role/my-role/some-session"}
	cases := map[string]struct {
		config     map[string]interface{}
		expVars    map[string]string
		expFilters []string
	}{
		"entity details disabled": {
			config: map[string]interface{}{
				"BoundIAMPrincipalARNs": principals,
			},
			expVars: map[string]string{
				"entity_name": "",
				"entity_id":   "",
				"account_id":  "",
			},
			expFilters: []string{
				`entity_name == ""`,
				`entity_id == ""`,
				`account_id == ""`,
			},
		},
		"entity details enabled": {
			config: map[string]interface{}{
				"BoundIAMPrincipalARNs":  principals,
				"EnableIAMEntityDetails": true,
			},
			expVars: map[string]string{
				"entity_name": "",
				"entity_id":   "",
				"account_id":  "",
				"entity_path": "",
			},
			expFilters: []string{
				`entity_name == ""`,
				`entity_id == ""`,
				`account_id == ""`,
				`entity_path == ""`,
			},
		},
		"entity tags": {
			config: map[string]interface{}{
				"BoundIAMPrincipalARNs":  principals,
				"EnableIAMEntityDetails": true,
				"IAMEntityTags": []string{
					"test_tag",
					"test_tag_2",
				},
			},
			expVars: map[string]string{
				"entity_name":            "",
				"entity_id":              "",
				"account_id":             "",
				"entity_path":            "",
				"entity_tags.test_tag":   "",
				"entity_tags.test_tag_2": "",
			},
			expFilters: []string{
				`entity_name == ""`,
				`entity_id == ""`,
				`account_id == ""`,
				`entity_path == ""`,
				`entity_tags.test_tag == ""`,
				`entity_tags.test_tag_2 == ""`,
			},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			method := &structs.ACLAuthMethod{
				Name:   "test-method",
				Type:   "aws-iam",
				Config: c.config,
			}
			nullLogger := hclog.NewNullLogger()
			v, err := NewValidator(nullLogger, method)
			require.NoError(t, err)

			id := v.NewIdentity()
			authmethod.RequireIdentityMatch(t, id, c.expVars, c.expFilters...)
		})
	}
}
