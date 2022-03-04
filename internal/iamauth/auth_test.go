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
		entityId  = "AAAsameuniqueid"
		accountId = "1234567890"

		serverForRole = &iamauthtest.Server{
			GetCallerIdentityResponse: responsestest.MakeGetCallerIdentityResponse(assumedRoleArn, entityId, accountId),
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
				STSEndpoint:         fakeAws.URL,
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
