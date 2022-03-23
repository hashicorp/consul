package iamauth

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/hashicorp/consul/internal/iamauth/iamauthtest"
	"github.com/hashicorp/consul/internal/iamauth/responses"
	"github.com/hashicorp/consul/internal/iamauth/responsestest"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
)

func TestValidateLogin(t *testing.T) {
	var (
		assumedRoleArn   = "arn:aws:sts::1234567890:assumed-role/my-role/some-session"
		canonicalRoleArn = "arn:aws:iam::1234567890:role/my-role"
		roleArnWithPath  = "arn:aws:iam::1234567890:role/some/path/my-role"
		// roleArnWildcard  = "arn:aws:iam::1234567890:role/some/path/*"
		// roleName = "my-role"
		userArn = "arn:aws:sts::1234567890:user/my-user"
		// userArnWildcard  = "arn:aws:sts::1234567890:user/*"
		// userName  = "my-user"
		entityId = "AAAsameuniqueid"
		// https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_variables.html#principaltable
		entityIdWithSession = entityId + ":some-session"
		accountId           = "1234567890"

		serverForRole = &iamauthtest.Server{
			GetCallerIdentityResponse: responsestest.MakeGetCallerIdentityResponse(assumedRoleArn, entityIdWithSession, accountId),
			GetRoleResponse: responsestest.MakeGetRoleResponse(
				roleArnWithPath,
				entityId,
				responses.Tag{Key: "service-name", Value: "my-service"},
				responses.Tag{Key: "env", Value: "my-env"},
			),
		}
		serverForUser = &iamauthtest.Server{
			GetCallerIdentityResponse: responsestest.MakeGetCallerIdentityResponse(userArn, entityId, accountId),
			GetUserResponse: responsestest.MakeGetUserResponse(
				userArn,
				entityId,
				responses.Tag{Key: "user-group", Value: "my-group"},
			),
		}
		serverForRoleMismatchedIds = &iamauthtest.Server{
			GetCallerIdentityResponse: responsestest.MakeGetCallerIdentityResponse(assumedRoleArn, entityIdWithSession, accountId),
			GetRoleResponse:           responsestest.MakeGetRoleResponse(roleArnWithPath, "AAAAsomenonmatchingid"),
		}
		serverForUserMismatchedIds = &iamauthtest.Server{
			GetCallerIdentityResponse: responsestest.MakeGetCallerIdentityResponse(userArn, entityId, accountId),
			GetUserResponse:           responsestest.MakeGetUserResponse(userArn, "AAAAsomenonmatchingid"),
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
			server:   serverForRole,
			config:   &Config{},
		},
		"no matching principal": {
			expError: "not trusted",
			server:   serverForUser,
			config: &Config{
				BoundIAMPrincipalARNs: []string{
					"arn:aws:iam::1234567890:user/some-other-role",
					"arn:aws:iam::1234567890:user/some-other-user",
				},
			},
		},
		"mismatched server id header": {
			expError: `expected "some-non-matching-value" but got "server.id.example.com"`,
			server:   serverForRole,
			config: &Config{
				BoundIAMPrincipalARNs: []string{canonicalRoleArn},
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
				BoundIAMPrincipalARNs:  []string{roleArnWithPath},
				EnableIAMEntityDetails: true,
			},
		},
		"user unique id mismatch": {
			expError: "unique id mismatch in login token",
			server:   serverForUserMismatchedIds,
			config: &Config{
				BoundIAMPrincipalARNs:  []string{userArn},
				EnableIAMEntityDetails: true,
			},
		},
	}
	logger := hclog.New(nil)
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			fakeAws := iamauthtest.NewTestServer(c.server)
			t.Cleanup(fakeAws.Close)
			fakeAws.Start()

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
