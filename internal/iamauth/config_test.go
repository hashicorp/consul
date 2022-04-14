package iamauth

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigValidate(t *testing.T) {
	principalArn := "arn:aws:iam::000000000000:role/my-role"

	cases := map[string]struct {
		expError string
		configs  []Config

		includeHeaderNames bool
	}{
		"bound iam principals are required": {
			expError: "BoundIAMPrincipalARNs is required and must have at least 1 entry",
			configs: []Config{
				{BoundIAMPrincipalARNs: nil},
				{BoundIAMPrincipalARNs: []string{}},
			},
		},
		"entity tags require entity details": {
			expError: "Must set EnableIAMEntityDetails=true to use IAMUserTags",
			configs: []Config{
				{
					BoundIAMPrincipalARNs:  []string{principalArn},
					EnableIAMEntityDetails: false,
					IAMEntityTags:          []string{"some-tag"},
				},
			},
		},
		"entity details require all entity header names": {
			expError: "Must set all of GetEntityMethodHeader, GetEntityURLHeader, " +
				"GetEntityHeadersHeader, and GetEntityBodyHeader when EnableIAMEntityDetails=true",
			configs: []Config{
				{
					BoundIAMPrincipalARNs:  []string{principalArn},
					EnableIAMEntityDetails: true,
				},
				{
					BoundIAMPrincipalARNs:  []string{principalArn},
					EnableIAMEntityDetails: true,
					GetEntityBodyHeader:    "X-Test-Header",
				},
				{
					BoundIAMPrincipalARNs:  []string{principalArn},
					EnableIAMEntityDetails: true,
					GetEntityHeadersHeader: "X-Test-Header",
				},
				{
					BoundIAMPrincipalARNs:  []string{principalArn},
					EnableIAMEntityDetails: true,
					GetEntityURLHeader:     "X-Test-Header",
				},
				{
					BoundIAMPrincipalARNs:  []string{principalArn},
					EnableIAMEntityDetails: true,
					GetEntityMethodHeader:  "X-Test-Header",
				},
			},
		},
		"wildcard principals require entity details": {
			expError: "Must set EnableIAMEntityDetails=true to use wildcards in BoundIAMPrincipalARNs",
			configs: []Config{
				{BoundIAMPrincipalARNs: []string{"arn:aws:iam::000000000000:role/*"}},
				{BoundIAMPrincipalARNs: []string{"arn:aws:iam::000000000000:role/path/*"}},
			},
		},
		"only one wildcard suffix is allowed": {
			expError: "Only one wildcard is allowed at the end of the bound IAM principal ARN",
			configs: []Config{
				{
					BoundIAMPrincipalARNs:  []string{"arn:aws:iam::000000000000:role/**"},
					EnableIAMEntityDetails: true,
				},
				{
					BoundIAMPrincipalARNs:  []string{"arn:aws:iam::000000000000:role/*/*"},
					EnableIAMEntityDetails: true,
				},
				{
					BoundIAMPrincipalARNs:  []string{"arn:aws:iam::000000000000:role/*/path"},
					EnableIAMEntityDetails: true,
				},
				{
					BoundIAMPrincipalARNs:  []string{"arn:aws:iam::000000000000:role/*/path/*"},
					EnableIAMEntityDetails: true,
				},
			},
		},
		"invalid principal arns are disallowed": {
			expError: fmt.Sprintf("Invalid principal ARN"),
			configs: []Config{
				{BoundIAMPrincipalARNs: []string{""}},
				{BoundIAMPrincipalARNs: []string{"  "}},
				{BoundIAMPrincipalARNs: []string{"*"}, EnableIAMEntityDetails: true},
				{BoundIAMPrincipalARNs: []string{"arn:aws:iam:role/my-role"}},
			},
		},
		"valid principal arns are allowed": {
			includeHeaderNames: true,
			configs: []Config{
				{BoundIAMPrincipalARNs: []string{"arn:aws:sts::000000000000:assumed-role/my-role/some-session-name"}},
				{BoundIAMPrincipalARNs: []string{"arn:aws:iam::000000000000:user/my-user"}},
				{BoundIAMPrincipalARNs: []string{"arn:aws:iam::000000000000:role/my-role"}},
				{BoundIAMPrincipalARNs: []string{"arn:aws:iam::000000000000:*"}, EnableIAMEntityDetails: true},
				{BoundIAMPrincipalARNs: []string{"arn:aws:iam::000000000000:role/*"}, EnableIAMEntityDetails: true},
				{BoundIAMPrincipalARNs: []string{"arn:aws:iam::000000000000:role/path/*"}, EnableIAMEntityDetails: true},
				{BoundIAMPrincipalARNs: []string{"arn:aws:iam::000000000000:user/*"}, EnableIAMEntityDetails: true},
				{BoundIAMPrincipalARNs: []string{"arn:aws:iam::000000000000:user/path/*"}, EnableIAMEntityDetails: true},
			},
		},
		"server id header value requires service id header name": {
			expError: "Must set ServerIDHeaderName to use a server ID value",
			configs: []Config{
				{
					BoundIAMPrincipalARNs: []string{principalArn},
					ServerIDHeaderValue:   "consul.test.example.com",
				},
			},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			for _, conf := range c.configs {
				if c.includeHeaderNames {
					setTestHeaderNames(&conf)
				}
				err := conf.Validate()
				if c.expError != "" {
					require.Error(t, err)
					require.Contains(t, err.Error(), c.expError)
				} else {
					require.NoError(t, err)
				}
			}
		})
	}
}

func setTestHeaderNames(conf *Config) {
	conf.GetEntityMethodHeader = "X-Test-Method"
	conf.GetEntityURLHeader = "X-Test-URL"
	conf.GetEntityHeadersHeader = "X-Test-Headers"
	conf.GetEntityBodyHeader = "X-Test-Body"
}
