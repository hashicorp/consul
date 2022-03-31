package iamauth

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/hashicorp/consul/internal/iamauth/iamauthtest"
	"github.com/hashicorp/consul/internal/iamauth/responsestest"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
)

func TestValidateLogin(t *testing.T) {
	f := iamauthtest.MakeFixture()

	var (
		serverForRoleMismatchedIds = &iamauthtest.Server{
			GetCallerIdentityResponse: f.ServerForRole.GetCallerIdentityResponse,
			GetRoleResponse:           responsestest.MakeGetRoleResponse(f.RoleARN, "AAAAsomenonmatchingid"),
		}
		serverForUserMismatchedIds = &iamauthtest.Server{
			GetCallerIdentityResponse: f.ServerForUser.GetCallerIdentityResponse,
			GetUserResponse:           responsestest.MakeGetUserResponse(f.UserARN, "AAAAsomenonmatchingid"),
		}
	)

	cases := map[string]struct {
		config   *Config
		server   *iamauthtest.Server
		expIdent *IdentityDetails
		expError string
	}{
		"no bound principals": {
			expError: "not trusted",
			server:   f.ServerForRole,
			config:   &Config{},
		},
		"no matching principal": {
			expError: "not trusted",
			server:   f.ServerForUser,
			config: &Config{
				BoundIAMPrincipalARNs: []string{
					"arn:aws:iam::1234567890:user/some-other-role",
					"arn:aws:iam::1234567890:user/some-other-user",
				},
			},
		},
		"mismatched server id header": {
			expError: `expected "some-non-matching-value" but got "server.id.example.com"`,
			server:   f.ServerForRole,
			config: &Config{
				BoundIAMPrincipalARNs: []string{f.CanonicalRoleARN},
				ServerIDHeaderValue:   "some-non-matching-value",
				ServerIDHeaderName:    "X-Test-ServerID",
			},
		},
		"role unique id mismatch": {
			expError: "unique id mismatch in login token",
			// The RoleId in the GetRole response must match the UserId in the GetCallerIdentity response
			// during login. If not, the RoleId cannot be used.
			server: serverForRoleMismatchedIds,
			config: &Config{
				BoundIAMPrincipalARNs:  []string{f.RoleARN},
				EnableIAMEntityDetails: true,
			},
		},
		"user unique id mismatch": {
			expError: "unique id mismatch in login token",
			server:   serverForUserMismatchedIds,
			config: &Config{
				BoundIAMPrincipalARNs:  []string{f.UserARN},
				EnableIAMEntityDetails: true,
			},
		},
	}
	logger := hclog.New(nil)
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			fakeAws := iamauthtest.NewTestServer(t, c.server)

			c.config.STSEndpoint = fakeAws.URL + "/sts"
			c.config.IAMEndpoint = fakeAws.URL + "/iam"
			setTestHeaderNames(c.config)

			// This bypasses NewAuthenticator, which bypasses config.Validate().
			auth := &Authenticator{config: c.config, logger: logger}

			loginInput := &LoginInput{
				Creds:               credentials.NewStaticCredentials("fake", "fake", ""),
				IncludeIAMEntity:    c.config.EnableIAMEntityDetails,
				STSEndpoint:         c.config.STSEndpoint,
				STSRegion:           "fake-region",
				Logger:              logger,
				ServerIDHeaderValue: "server.id.example.com",
			}
			setLoginInputHeaderNames(loginInput)
			loginData, err := GenerateLoginData(loginInput)
			require.NoError(t, err)
			loginBytes, err := json.Marshal(loginData)
			require.NoError(t, err)

			ident, err := auth.ValidateLogin(context.Background(), string(loginBytes))
			if c.expError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), c.expError)
				require.Nil(t, ident)
			} else {
				require.NoError(t, err)
				require.Equal(t, c.expIdent, ident)
			}
		})
	}
}

func setLoginInputHeaderNames(in *LoginInput) {
	in.ServerIDHeaderName = "X-Test-ServerID"
	in.GetEntityMethodHeader = "X-Test-Method"
	in.GetEntityURLHeader = "X-Test-URL"
	in.GetEntityHeadersHeader = "X-Test-Headers"
	in.GetEntityBodyHeader = "X-Test-Body"
}
