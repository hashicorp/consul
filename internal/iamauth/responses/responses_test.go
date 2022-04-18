package responses

import (
	"encoding/xml"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseArn(t *testing.T) {
	cases := map[string]struct {
		arn    string
		expArn *ParsedArn
	}{
		"assumed-role": {
			arn: "arn:aws:sts::000000000000:assumed-role/my-role/session-name",
			expArn: &ParsedArn{
				Partition:     "aws",
				AccountNumber: "000000000000",
				Type:          "assumed-role",
				Path:          "",
				FriendlyName:  "my-role",
				SessionInfo:   "session-name",
			},
		},
		"role": {
			arn: "arn:aws:iam::000000000000:role/my-role",
			expArn: &ParsedArn{
				Partition:     "aws",
				AccountNumber: "000000000000",
				Type:          "role",
				Path:          "",
				FriendlyName:  "my-role",
				SessionInfo:   "",
			},
		},
		"user": {
			arn: "arn:aws:iam::000000000000:user/my-user",
			expArn: &ParsedArn{
				Partition:     "aws",
				AccountNumber: "000000000000",
				Type:          "user",
				Path:          "",
				FriendlyName:  "my-user",
				SessionInfo:   "",
			},
		},
		"role with path": {
			arn: "arn:aws:iam::000000000000:role/path/my-role",
			expArn: &ParsedArn{
				Partition:     "aws",
				AccountNumber: "000000000000",
				Type:          "role",
				Path:          "path",
				FriendlyName:  "my-role",
				SessionInfo:   "",
			},
		},
		"role with path 2": {
			arn: "arn:aws:iam::000000000000:role/path/to/my-role",
			expArn: &ParsedArn{
				Partition:     "aws",
				AccountNumber: "000000000000",
				Type:          "role",
				Path:          "path/to",
				FriendlyName:  "my-role",
				SessionInfo:   "",
			},
		},
		"role with path 3": {
			arn: "arn:aws:iam::000000000000:role/some/path/to/my-role",
			expArn: &ParsedArn{
				Partition:     "aws",
				AccountNumber: "000000000000",
				Type:          "role",
				Path:          "some/path/to",
				FriendlyName:  "my-role",
				SessionInfo:   "",
			},
		},
		"user with path": {
			arn: "arn:aws:iam::000000000000:user/path/my-user",
			expArn: &ParsedArn{
				Partition:     "aws",
				AccountNumber: "000000000000",
				Type:          "user",
				Path:          "path",
				FriendlyName:  "my-user",
				SessionInfo:   "",
			},
		},

		// Invalid cases
		"empty string":               {arn: ""},
		"wildcard":                   {arn: "*"},
		"missing prefix":             {arn: ":aws:sts::000000000000:assumed-role/my-role/session-name"},
		"missing partition":          {arn: "arn::sts::000000000000:assumed-role/my-role/session-name"},
		"missing service":            {arn: "arn:aws:::000000000000:assumed-role/my-role/session-name"},
		"missing separator":          {arn: "arn:aws:sts:000000000000:assumed-role/my-role/session-name"},
		"missing account id":         {arn: "arn:aws:sts:::assumed-role/my-role/session-name"},
		"missing resource":           {arn: "arn:aws:sts::000000000000:"},
		"assumed-role missing parts": {arn: "arn:aws:sts::000000000000:assumed-role/my-role"},
		"role missing parts":         {arn: "arn:aws:sts::000000000000:role"},
		"role missing parts 2":       {arn: "arn:aws:sts::000000000000:role/"},
		"user missing parts":         {arn: "arn:aws:sts::000000000000:user"},
		"user missing parts 2":       {arn: "arn:aws:sts::000000000000:user/"},
		"unsupported service":        {arn: "arn:aws:ecs:us-east-1:000000000000:task/my-task/00000000000000000000000000000000"},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			parsed, err := ParseArn(c.arn)
			if c.expArn != nil {
				require.NoError(t, err)
				require.Equal(t, c.expArn, parsed)
			} else {
				require.Error(t, err)
				require.Nil(t, parsed)
			}
		})
	}
}

func TestCanonicalArn(t *testing.T) {
	cases := map[string]struct {
		arn    string
		expArn string
	}{
		"assumed-role arn": {
			arn:    "arn:aws:sts::000000000000:assumed-role/my-role/session-name",
			expArn: "arn:aws:iam::000000000000:role/my-role",
		},
		"role arn": {
			arn:    "arn:aws:iam::000000000000:role/my-role",
			expArn: "arn:aws:iam::000000000000:role/my-role",
		},
		"role arn with path": {
			arn:    "arn:aws:iam::000000000000:role/path/to/my-role",
			expArn: "arn:aws:iam::000000000000:role/my-role",
		},
		"user arn": {
			arn:    "arn:aws:iam::000000000000:user/my-user",
			expArn: "arn:aws:iam::000000000000:user/my-user",
		},
		"user arn with path": {
			arn:    "arn:aws:iam::000000000000:user/path/to/my-user",
			expArn: "arn:aws:iam::000000000000:user/my-user",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			parsed, err := ParseArn(c.arn)
			require.NoError(t, err)
			require.Equal(t, c.expArn, parsed.CanonicalArn())
		})
	}
}

func TestUnmarshalXML(t *testing.T) {
	t.Run("user xml", func(t *testing.T) {
		var resp GetUserResponse
		err := xml.Unmarshal([]byte(rawUserXML), &resp)
		require.NoError(t, err)
		require.Equal(t, expectedParsedUserXML, resp)
	})
	t.Run("role xml", func(t *testing.T) {
		var resp GetRoleResponse
		err := xml.Unmarshal([]byte(rawRoleXML), &resp)
		require.NoError(t, err)
		require.Equal(t, expectedParsedRoleXML, resp)
	})
}

var (
	rawUserXML = `<GetUserResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
  <GetUserResult>
	<User>
	  <Path>/</Path>
	  <Arn>arn:aws:iam::000000000000:user/my-user</Arn>
	  <UserName>my-user</UserName>
	  <UserId>AIDAexampleuserid</UserId>
	  <CreateDate>2021-01-01T00:01:02Z</CreateDate>
	  <Tags>
		<member>
		  <Value>some-value</Value>
		  <Key>some-tag</Key>
		</member>
		<member>
		  <Value>another-value</Value>
		  <Key>another-tag</Key>
		</member>
		<member>
		  <Value>third-value</Value>
		  <Key>third-tag</Key>
		</member>
	  </Tags>
	</User>
  </GetUserResult>
  <ResponseMetadata>
	<RequestId>11815b96-cb16-4d33-b2cf-0042fa4db4cd</RequestId>
  </ResponseMetadata>
</GetUserResponse>`

	expectedParsedUserXML = GetUserResponse{
		XMLName: xml.Name{
			Space: "https://iam.amazonaws.com/doc/2010-05-08/",
			Local: "GetUserResponse",
		},
		GetUserResult: []GetUserResult{
			{
				User: User{
					Arn:      "arn:aws:iam::000000000000:user/my-user",
					Path:     "/",
					UserId:   "AIDAexampleuserid",
					UserName: "my-user",
					Tags: Tags{
						Members: []TagMember{
							{Key: "some-tag", Value: "some-value"},
							{Key: "another-tag", Value: "another-value"},
							{Key: "third-tag", Value: "third-value"},
						},
					},
				},
			},
		},
		ResponseMetadata: []ResponseMetadata{
			{RequestId: "11815b96-cb16-4d33-b2cf-0042fa4db4cd"},
		},
	}

	rawRoleXML = `<GetRoleResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
  <GetRoleResult>
    <Role>
      <Path>/</Path>
      <AssumeRolePolicyDocument>some-json-document-that-we-ignore</AssumeRolePolicyDocument>
      <MaxSessionDuration>43200</MaxSessionDuration>
      <RoleId>AROAsomeuniqueid</RoleId>
      <RoleLastUsed>
        <LastUsedDate>2022-01-01T01:02:03Z</LastUsedDate>
        <Region>us-east-1</Region>
      </RoleLastUsed>
      <RoleName>my-role</RoleName>
      <Arn>arn:aws:iam::000000000000:role/my-role</Arn>
      <CreateDate>2020-01-01T00:00:01Z</CreateDate>
      <Tags>
        <member>
          <Value>some-value</Value>
          <Key>some-key</Key>
        </member>
        <member>
          <Value>another-value</Value>
          <Key>another-key</Key>
        </member>
        <member>
          <Value>a-third-value</Value>
          <Key>third-key</Key>
        </member>
      </Tags>
    </Role>
  </GetRoleResult>
  <ResponseMetadata>
    <RequestId>a9866067-c0e5-4b5e-86ba-429c1151e2fb</RequestId>
  </ResponseMetadata>
</GetRoleResponse>`

	expectedParsedRoleXML = GetRoleResponse{
		XMLName: xml.Name{
			Space: "https://iam.amazonaws.com/doc/2010-05-08/",
			Local: "GetRoleResponse",
		},
		GetRoleResult: []GetRoleResult{
			{
				Role: Role{
					Arn:      "arn:aws:iam::000000000000:role/my-role",
					Path:     "/",
					RoleId:   "AROAsomeuniqueid",
					RoleName: "my-role",
					Tags: Tags{
						Members: []TagMember{
							{Key: "some-key", Value: "some-value"},
							{Key: "another-key", Value: "another-value"},
							{Key: "third-key", Value: "a-third-value"},
						},
					},
				},
			},
		},
		ResponseMetadata: []ResponseMetadata{
			{RequestId: "a9866067-c0e5-4b5e-86ba-429c1151e2fb"},
		},
	}
)
