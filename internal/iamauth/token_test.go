package iamauth

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewBearerToken(t *testing.T) {
	cases := map[string]struct {
		tokenStr string
		config   Config
		expToken BearerToken
		expError string
	}{
		"valid token": {
			tokenStr: validBearerTokenJson,
			expToken: validBearerTokenParsed,
		},
		"valid token with role": {
			tokenStr: validBearerTokenWithRoleJson,
			config: Config{
				EnableIAMEntityDetails: true,
				GetEntityMethodHeader:  "X-Consul-IAM-GetEntity-Method",
				GetEntityURLHeader:     "X-Consul-IAM-GetEntity-URL",
				GetEntityHeadersHeader: "X-Consul-IAM-GetEntity-Headers",
				GetEntityBodyHeader:    "X-Consul-IAM-GetEntity-Body",
				STSEndpoint:            validBearerTokenParsed.getCallerIdentityURL,
			},
			expToken: validBearerTokenWithRoleParsed,
		},

		"empty json": {
			tokenStr: `{}`,
			expError: "unexpected end of JSON input",
		},
		"missing iam_request_method field": {
			tokenStr: tokenJsonMissingMethodField,
			expError: "iam_http_request_method must be POST",
		},
		"missing iam_request_url field": {
			tokenStr: tokenJsonMissingUrlField,
			expError: "url is invalid",
		},
		"missing iam_request_headers field": {
			tokenStr: tokenJsonMissingHeadersField,
			expError: "unexpected end of JSON input",
		},
		"missing iam_request_body field": {
			tokenStr: tokenJsonMissingBodyField,
			expError: "iam_request_body error",
		},
		"invalid json": {
			tokenStr: `{`,
			expError: "unexpected end of JSON input",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			token, err := NewBearerToken(c.tokenStr, &c.config)
			t.Logf("token = %+v", token)
			if c.expError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), c.expError)
				require.Nil(t, token)
			} else {
				require.NoError(t, err)
				c.expToken.config = &c.config
				require.Equal(t, &c.expToken, token)
			}
		})
	}
}

func TestParseRequestBody(t *testing.T) {
	cases := map[string]struct {
		body          string
		allowedValues url.Values
		expValues     url.Values
		expError      string
	}{
		"one allowed field": {
			body:          "Action=GetCallerIdentity&Version=1234",
			allowedValues: url.Values{"Version": []string{"1234"}},
			expValues: url.Values{
				"Action":  []string{"GetCallerIdentity"},
				"Version": []string{"1234"},
			},
		},
		"many allowed fields": {
			body: "Action=GetRole&RoleName=my-role&Version=1234",
			allowedValues: url.Values{
				"Action":   []string{"GetUser", "GetRole"},
				"UserName": nil,
				"RoleName": nil,
				"Version":  nil,
			},
			expValues: url.Values{
				"Action":   []string{"GetRole"},
				"RoleName": []string{"my-role"},
				"Version":  []string{"1234"},
			},
		},
		"action only": {
			body:          "Action=GetRole",
			allowedValues: nil,
			expValues:     url.Values{"Action": []string{"GetRole"}},
		},

		"empty body": {
			expValues: url.Values{},
			expError:  `missing field "Action"`,
		},
		"disallowed field": {
			body:          "Action=GetRole&Version=1234&Extra=Abc",
			allowedValues: url.Values{"Action": nil, "Version": nil},
			expError:      `unexpected field "Extra"`,
		},
		"mismatched action": {
			body:          "Action=GetRole",
			allowedValues: url.Values{"Action": []string{"GetUser"}},
			expError:      `unexpected value Action=[GetRole]`,
		},
		"mismatched field": {
			body:          "Action=GetRole&Extra=1234",
			allowedValues: url.Values{"Action": nil, "Extra": []string{"abc"}},
			expError:      `unexpected value Extra=[1234]`,
		},
		"multi-valued field": {
			body:          "Action=GetRole&Action=GetUser",
			allowedValues: url.Values{"Action": []string{"GetRole", "GetUser"}},
			// only one value is allowed.
			expError: `unexpected value Action=[GetRole GetUser]`,
		},
		"empty action": {
			body:          "Action=",
			allowedValues: nil,
			expError:      `missing field "Action"`,
		},
		"missing action": {
			body:          "Version=1234",
			allowedValues: url.Values{"Action": []string{"GetRole"}},
			expError:      `missing field "Action"`,
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			values, err := parseRequestBody(c.body, c.allowedValues)
			if c.expError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), c.expError)
				require.Nil(t, values)
			} else {
				require.NoError(t, err)
				require.Equal(t, c.expValues, values)
			}
		})
	}
}

func TestValidateGetCallerIdentityBody(t *testing.T) {
	cases := map[string]struct {
		body     string
		expError string
	}{
		"valid":   {"Action=GetCallerIdentity&Version=1234", ""},
		"valid 2": {"Action=GetCallerIdentity", ""},
		"empty action": {
			"Action=",
			`iam_request_body error: missing field "Action"`,
		},
		"invalid action": {
			"Action=GetRole",
			`iam_request_body error: unexpected value Action=[GetRole]`,
		},
		"missing action": {
			"Version=1234",
			`iam_request_body error: missing field "Action"`,
		},
		"empty": {
			"",
			`iam_request_body error: missing field "Action"`,
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			token := &BearerToken{getCallerIdentityBody: c.body}
			err := token.validateGetCallerIdentityBody()
			if c.expError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), c.expError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateIAMEntityBody(t *testing.T) {
	cases := map[string]struct {
		body       string
		expReqType string
		expError   string
	}{
		"valid role": {
			body:       "Action=GetRole&RoleName=my-role&Version=1234",
			expReqType: "GetRole",
		},
		"valid role without version": {
			body:       "Action=GetRole&RoleName=my-role",
			expReqType: "GetRole",
		},
		"valid user": {
			body:       "Action=GetUser&UserName=my-role&Version=1234",
			expReqType: "GetUser",
		},
		"valid user without version": {
			body:       "Action=GetUser&UserName=my-role",
			expReqType: "GetUser",
		},

		"invalid action": {
			body:     "Action=GetCallerIdentity",
			expError: `unexpected value Action=[GetCallerIdentity]`,
		},
		"role missing action": {
			body:     "RoleName=my-role&Version=1234",
			expError: `missing field "Action"`,
		},
		"user missing action": {
			body:     "UserName=my-role&Version=1234",
			expError: `missing field "Action"`,
		},
		"empty": {
			body:     "",
			expError: `missing field "Action"`,
		},
		"empty action": {
			body:     "Action=",
			expError: `missing field "Action"`,
		},
		"role with user name": {
			body:     "Action=GetRole&UserName=my-role&Version=1234",
			expError: `invalid request body`,
		},
		"user with role name": {
			body:     "Action=GetUser&RoleName=my-role&Version=1234",
			expError: `invalid request body`,
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			token := &BearerToken{
				config:           &Config{},
				getIAMEntityBody: c.body,
			}
			reqType, err := token.validateIAMEntityBody()
			if c.expError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), c.expError)
				require.Equal(t, "", reqType)
			} else {
				require.NoError(t, err)
				require.Equal(t, c.expReqType, reqType)
			}
		})
	}
}

func TestValidateSTSHostname(t *testing.T) {
	cases := []struct {
		url string
		ok  bool
	}{
		// https://docs.aws.amazon.com/general/latest/gr/sts.html
		{"sts.us-east-2.amazonaws.com", true},
		{"sts-fips.us-east-2.amazonaws.com", true},
		{"sts.us-east-1.amazonaws.com", true},
		{"sts-fips.us-east-1.amazonaws.com", true},
		{"sts.us-west-1.amazonaws.com", true},
		{"sts-fips.us-west-1.amazonaws.com", true},
		{"sts.us-west-2.amazonaws.com", true},
		{"sts-fips.us-west-2.amazonaws.com", true},
		{"sts.af-south-1.amazonaws.com", true},
		{"sts.ap-east-1.amazonaws.com", true},
		{"sts.ap-southeast-3.amazonaws.com", true},
		{"sts.ap-south-1.amazonaws.com", true},
		{"sts.ap-northeast-3.amazonaws.com", true},
		{"sts.ap-northeast-2.amazonaws.com", true},
		{"sts.ap-southeast-1.amazonaws.com", true},
		{"sts.ap-southeast-2.amazonaws.com", true},
		{"sts.ap-northeast-1.amazonaws.com", true},
		{"sts.ca-central-1.amazonaws.com", true},
		{"sts.eu-central-1.amazonaws.com", true},
		{"sts.eu-west-1.amazonaws.com", true},
		{"sts.eu-west-2.amazonaws.com", true},
		{"sts.eu-south-1.amazonaws.com", true},
		{"sts.eu-west-3.amazonaws.com", true},
		{"sts.eu-north-1.amazonaws.com", true},
		{"sts.me-south-1.amazonaws.com", true},
		{"sts.sa-east-1.amazonaws.com", true},
		{"sts.us-gov-east-1.amazonaws.com", true},
		{"sts.us-gov-west-1.amazonaws.com", true},

		// prefix must be either 'sts.' or 'sts-fips.'
		{".amazonaws.com", false},
		{"iam.amazonaws.com", false},
		{"other.amazonaws.com", false},
		// suffix must be '.amazonaws.com' and not some other domain
		{"stsamazonaws.com", false},
		{"sts-fipsamazonaws.com", false},
		{"sts.stsamazonaws.com", false},
		{"sts.notamazonaws.com", false},
		{"sts-fips.stsamazonaws.com", false},
		{"sts-fips.notamazonaws.com", false},
		{"sts.amazonaws.com.spoof", false},
		{"sts.amazonaws.spoof.com", false},
		{"xyz.sts.amazonaws.com", false},
	}
	for _, c := range cases {
		t.Run(c.url, func(t *testing.T) {
			url := "https://" + c.url
			parsedUrl, err := parseUrl(url)
			require.NoError(t, err)

			token := &BearerToken{
				config:                  &Config{},
				getCallerIdentityURL:    url,
				parsedCallerIdentityURL: parsedUrl,
			}
			err = token.validateSTSHostname()
			if c.ok {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestValidateIAMHostname(t *testing.T) {
	cases := []struct {
		url string
		ok  bool
	}{
		// https://docs.aws.amazon.com/general/latest/gr/iam-service.html
		{"iam.amazonaws.com", true},
		{"iam-fips.amazonaws.com", true},
		{"iam.us-gov.amazonaws.com", true},
		{"iam-fips.us-gov.amazonaws.com", true},

		// prefix must be either 'iam.' or 'aim-fips.'
		{".amazonaws.com", false},
		{"sts.amazonaws.com", false},
		{"other.amazonaws.com", false},
		// suffix must be '.amazonaws.com' and not some other domain
		{"iamamazonaws.com", false},
		{"iam-fipsamazonaws.com", false},
		{"iam.iamamazonaws.com", false},
		{"iam.notamazonaws.com", false},
		{"iam-fips.iamamazonaws.com", false},
		{"iam-fips.notamazonaws.com", false},
		{"iam.amazonaws.com.spoof", false},
		{"iam.amazonaws.spoof.com", false},
		{"xyz.iam.amazonaws.com", false},
	}
	for _, c := range cases {
		t.Run(c.url, func(t *testing.T) {
			url := "https://" + c.url
			parsedUrl, err := parseUrl(url)
			require.NoError(t, err)

			token := &BearerToken{
				config:               &Config{},
				getCallerIdentityURL: url,
				parsedIAMEntityURL:   parsedUrl,
			}
			err = token.validateIAMHostname()
			if c.ok {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

var (
	validBearerTokenJson = `{
  "iam_http_request_method":"POST",
  "iam_request_body":"QWN0aW9uPUdldENhbGxlcklkZW50aXR5JlZlcnNpb249MjAxMS0wNi0xNQ==",
  "iam_request_headers":"eyJBdXRob3JpemF0aW9uIjpbIkFXUzQtSE1BQy1TSEEyNTYgQ3JlZGVudGlhbD1mYWtlLzIwMjIwMzIyL3VzLWVhc3QtMS9zdHMvYXdzNF9yZXF1ZXN0LCBTaWduZWRIZWFkZXJzPWNvbnRlbnQtbGVuZ3RoO2NvbnRlbnQtdHlwZTtob3N0O3gtYW16LWRhdGU7eC1hbXotc2VjdXJpdHktdG9rZW4sIFNpZ25hdHVyZT1lZmMzMjBiOTcyZDA3YjM4YjY1ZWIyNDI1NjgwNWUwMzE0OWRhNTg2ZDgwNGY4YzYzNjRjZTk4ZGViZTA4MGIxIl0sIkNvbnRlbnQtTGVuZ3RoIjpbIjQzIl0sIkNvbnRlbnQtVHlwZSI6WyJhcHBsaWNhdGlvbi94LXd3dy1mb3JtLXVybGVuY29kZWQ7IGNoYXJzZXQ9dXRmLTgiXSwiVXNlci1BZ2VudCI6WyJhd3Mtc2RrLWdvLzEuNDIuMzQgKGdvMS4xNy41OyBkYXJ3aW47IGFtZDY0KSJdLCJYLUFtei1EYXRlIjpbIjIwMjIwMzIyVDIxMTEwM1oiXSwiWC1BbXotU2VjdXJpdHktVG9rZW4iOlsiZmFrZSJdfQ==",
  "iam_request_url":"aHR0cHM6Ly9zdHMuYW1hem9uYXdzLmNvbS8="
}`

	validBearerTokenParsed = BearerToken{
		getCallerIdentityMethod: "POST",
		getCallerIdentityURL:    "https://sts.amazonaws.com/",
		getCallerIdentityHeader: http.Header{
			"Authorization":        []string{"AWS4-HMAC-SHA256 Credential=fake/20220322/us-east-1/sts/aws4_request, SignedHeaders=content-length;content-type;host;x-amz-date;x-amz-security-token, Signature=efc320b972d07b38b65eb24256805e03149da586d804f8c6364ce98debe080b1"},
			"Content-Length":       []string{"43"},
			"Content-Type":         []string{"application/x-www-form-urlencoded; charset=utf-8"},
			"User-Agent":           []string{"aws-sdk-go/1.42.34 (go1.17.5; darwin; amd64)"},
			"X-Amz-Date":           []string{"20220322T211103Z"},
			"X-Amz-Security-Token": []string{"fake"},
		},
		getCallerIdentityBody: "Action=GetCallerIdentity&Version=2011-06-15",
		parsedCallerIdentityURL: &url.URL{
			Scheme: "https",
			Host:   "sts.amazonaws.com",
			Path:   "/",
		},
	}

	validBearerTokenWithRoleJson = `{"iam_http_request_method":"POST","iam_request_body":"QWN0aW9uPUdldENhbGxlcklkZW50aXR5JlZlcnNpb249MjAxMS0wNi0xNQ==","iam_request_headers":"eyJBdXRob3JpemF0aW9uIjpbIkFXUzQtSE1BQy1TSEEyNTYgQ3JlZGVudGlhbD1mYWtlLWtleS1pZC8yMDIyMDMyMi9mYWtlLXJlZ2lvbi9zdHMvYXdzNF9yZXF1ZXN0LCBTaWduZWRIZWFkZXJzPWNvbnRlbnQtbGVuZ3RoO2NvbnRlbnQtdHlwZTtob3N0O3gtYW16LWRhdGU7eC1jb25zdWwtaWFtLWdldGVudGl0eS1ib2R5O3gtY29uc3VsLWlhbS1nZXRlbnRpdHktaGVhZGVyczt4LWNvbnN1bC1pYW0tZ2V0ZW50aXR5LW1ldGhvZDt4LWNvbnN1bC1pYW0tZ2V0ZW50aXR5LXVybCwgU2lnbmF0dXJlPTU2MWFjMzFiNWFkMDFjMTI0YzU0YzE2OGY3NmVhNmJmZDY0NWI4ZWM1MzQ1ZjgzNTc3MjljOWFhMGI0NzEzMzciXSwiQ29udGVudC1MZW5ndGgiOlsiNDMiXSwiQ29udGVudC1UeXBlIjpbImFwcGxpY2F0aW9uL3gtd3d3LWZvcm0tdXJsZW5jb2RlZDsgY2hhcnNldD11dGYtOCJdLCJVc2VyLUFnZW50IjpbImF3cy1zZGstZ28vMS40Mi4zNCAoZ28xLjE3LjU7IGRhcndpbjsgYW1kNjQpIl0sIlgtQW16LURhdGUiOlsiMjAyMjAzMjJUMjI1NzQyWiJdLCJYLUNvbnN1bC1JYW0tR2V0ZW50aXR5LUJvZHkiOlsiQWN0aW9uPUdldFJvbGVcdTAwMjZSb2xlTmFtZT1teS1yb2xlXHUwMDI2VmVyc2lvbj0yMDEwLTA1LTA4Il0sIlgtQ29uc3VsLUlhbS1HZXRlbnRpdHktSGVhZGVycyI6WyJ7XCJBdXRob3JpemF0aW9uXCI6W1wiQVdTNC1ITUFDLVNIQTI1NiBDcmVkZW50aWFsPWZha2Uta2V5LWlkLzIwMjIwMzIyL3VzLWVhc3QtMS9pYW0vYXdzNF9yZXF1ZXN0LCBTaWduZWRIZWFkZXJzPWNvbnRlbnQtbGVuZ3RoO2NvbnRlbnQtdHlwZTtob3N0O3gtYW16LWRhdGUsIFNpZ25hdHVyZT1hYTJhMTlkMGEzMDVkNzRiYmQwMDk3NzZiY2E4ODBlNTNjZmE5OTFlNDgzZTQwMzk0NzE4MWE0MWNjNDgyOTQwXCJdLFwiQ29udGVudC1MZW5ndGhcIjpbXCI1MFwiXSxcIkNvbnRlbnQtVHlwZVwiOltcImFwcGxpY2F0aW9uL3gtd3d3LWZvcm0tdXJsZW5jb2RlZDsgY2hhcnNldD11dGYtOFwiXSxcIlVzZXItQWdlbnRcIjpbXCJhd3Mtc2RrLWdvLzEuNDIuMzQgKGdvMS4xNy41OyBkYXJ3aW47IGFtZDY0KVwiXSxcIlgtQW16LURhdGVcIjpbXCIyMDIyMDMyMlQyMjU3NDJaXCJdfSJdLCJYLUNvbnN1bC1JYW0tR2V0ZW50aXR5LU1ldGhvZCI6WyJQT1NUIl0sIlgtQ29uc3VsLUlhbS1HZXRlbnRpdHktVXJsIjpbImh0dHBzOi8vaWFtLmFtYXpvbmF3cy5jb20vIl19","iam_request_url":"aHR0cDovLzEyNy4wLjAuMTo2MzY5Ni9zdHMv"}`

	validBearerTokenWithRoleParsed = BearerToken{
		getCallerIdentityMethod: "POST",
		getCallerIdentityURL:    "http://127.0.0.1:63696/sts/",
		getCallerIdentityHeader: http.Header{
			"Authorization":                  []string{"AWS4-HMAC-SHA256 Credential=fake-key-id/20220322/fake-region/sts/aws4_request, SignedHeaders=content-length;content-type;host;x-amz-date;x-consul-iam-getentity-body;x-consul-iam-getentity-headers;x-consul-iam-getentity-method;x-consul-iam-getentity-url, Signature=561ac31b5ad01c124c54c168f76ea6bfd645b8ec5345f8357729c9aa0b471337"},
			"Content-Length":                 []string{"43"},
			"Content-Type":                   []string{"application/x-www-form-urlencoded; charset=utf-8"},
			"User-Agent":                     []string{"aws-sdk-go/1.42.34 (go1.17.5; darwin; amd64)"},
			"X-Amz-Date":                     []string{"20220322T225742Z"},
			"X-Consul-Iam-Getentity-Body":    []string{"Action=GetRole&RoleName=my-role&Version=2010-05-08"},
			"X-Consul-Iam-Getentity-Headers": []string{`{"Authorization":["AWS4-HMAC-SHA256 Credential=fake-key-id/20220322/us-east-1/iam/aws4_request, SignedHeaders=content-length;content-type;host;x-amz-date, Signature=aa2a19d0a305d74bbd009776bca880e53cfa991e483e403947181a41cc482940"],"Content-Length":["50"],"Content-Type":["application/x-www-form-urlencoded; charset=utf-8"],"User-Agent":["aws-sdk-go/1.42.34 (go1.17.5; darwin; amd64)"],"X-Amz-Date":["20220322T225742Z"]}`},
			"X-Consul-Iam-Getentity-Method":  []string{"POST"},
			"X-Consul-Iam-Getentity-Url":     []string{"https://iam.amazonaws.com/"},
		},
		getCallerIdentityBody: "Action=GetCallerIdentity&Version=2011-06-15",

		// Fields parsed from headers above
		getIAMEntityMethod: "POST",
		getIAMEntityURL:    "https://iam.amazonaws.com/",
		getIAMEntityHeader: http.Header{
			"Authorization":  []string{"AWS4-HMAC-SHA256 Credential=fake-key-id/20220322/us-east-1/iam/aws4_request, SignedHeaders=content-length;content-type;host;x-amz-date, Signature=aa2a19d0a305d74bbd009776bca880e53cfa991e483e403947181a41cc482940"},
			"Content-Length": []string{"50"},
			"Content-Type":   []string{"application/x-www-form-urlencoded; charset=utf-8"},
			"User-Agent":     []string{"aws-sdk-go/1.42.34 (go1.17.5; darwin; amd64)"},
			"X-Amz-Date":     []string{"20220322T225742Z"},
		},
		getIAMEntityBody:  "Action=GetRole&RoleName=my-role&Version=2010-05-08",
		entityRequestType: "GetRole",

		parsedCallerIdentityURL: &url.URL{
			Scheme: "http",
			Host:   "127.0.0.1:63696",
			Path:   "/sts/",
		},
		parsedIAMEntityURL: &url.URL{
			Scheme: "https",
			Host:   "iam.amazonaws.com",
			Path:   "/",
		},
	}

	tokenJsonMissingMethodField = `{
  "iam_request_body":"QWN0aW9uPUdldENhbGxlcklkZW50aXR5JlZlcnNpb249MjAxMS0wNi0xNQ==",
  "iam_request_headers":"eyJBdXRob3JpemF0aW9uIjpbIkFXUzQtSE1BQy1TSEEyNTYgQ3JlZGVudGlhbD1mYWtlLzIwMjIwMzIyL3VzLWVhc3QtMS9zdHMvYXdzNF9yZXF1ZXN0LCBTaWduZWRIZWFkZXJzPWNvbnRlbnQtbGVuZ3RoO2NvbnRlbnQtdHlwZTtob3N0O3gtYW16LWRhdGU7eC1hbXotc2VjdXJpdHktdG9rZW4sIFNpZ25hdHVyZT1lZmMzMjBiOTcyZDA3YjM4YjY1ZWIyNDI1NjgwNWUwMzE0OWRhNTg2ZDgwNGY4YzYzNjRjZTk4ZGViZTA4MGIxIl0sIkNvbnRlbnQtTGVuZ3RoIjpbIjQzIl0sIkNvbnRlbnQtVHlwZSI6WyJhcHBsaWNhdGlvbi94LXd3dy1mb3JtLXVybGVuY29kZWQ7IGNoYXJzZXQ9dXRmLTgiXSwiVXNlci1BZ2VudCI6WyJhd3Mtc2RrLWdvLzEuNDIuMzQgKGdvMS4xNy41OyBkYXJ3aW47IGFtZDY0KSJdLCJYLUFtei1EYXRlIjpbIjIwMjIwMzIyVDIxMTEwM1oiXSwiWC1BbXotU2VjdXJpdHktVG9rZW4iOlsiZmFrZSJdfQ==",
  "iam_request_url":"aHR0cHM6Ly9zdHMuYW1hem9uYXdzLmNvbS8="
}`

	tokenJsonMissingBodyField = `{
  "iam_http_request_method":"POST",
  "iam_request_headers":"eyJBdXRob3JpemF0aW9uIjpbIkFXUzQtSE1BQy1TSEEyNTYgQ3JlZGVudGlhbD1mYWtlLzIwMjIwMzIyL3VzLWVhc3QtMS9zdHMvYXdzNF9yZXF1ZXN0LCBTaWduZWRIZWFkZXJzPWNvbnRlbnQtbGVuZ3RoO2NvbnRlbnQtdHlwZTtob3N0O3gtYW16LWRhdGU7eC1hbXotc2VjdXJpdHktdG9rZW4sIFNpZ25hdHVyZT1lZmMzMjBiOTcyZDA3YjM4YjY1ZWIyNDI1NjgwNWUwMzE0OWRhNTg2ZDgwNGY4YzYzNjRjZTk4ZGViZTA4MGIxIl0sIkNvbnRlbnQtTGVuZ3RoIjpbIjQzIl0sIkNvbnRlbnQtVHlwZSI6WyJhcHBsaWNhdGlvbi94LXd3dy1mb3JtLXVybGVuY29kZWQ7IGNoYXJzZXQ9dXRmLTgiXSwiVXNlci1BZ2VudCI6WyJhd3Mtc2RrLWdvLzEuNDIuMzQgKGdvMS4xNy41OyBkYXJ3aW47IGFtZDY0KSJdLCJYLUFtei1EYXRlIjpbIjIwMjIwMzIyVDIxMTEwM1oiXSwiWC1BbXotU2VjdXJpdHktVG9rZW4iOlsiZmFrZSJdfQ==",
  "iam_request_url":"aHR0cHM6Ly9zdHMuYW1hem9uYXdzLmNvbS8="
}`

	tokenJsonMissingHeadersField = `{
  "iam_http_request_method":"POST",
  "iam_request_body":"QWN0aW9uPUdldENhbGxlcklkZW50aXR5JlZlcnNpb249MjAxMS0wNi0xNQ==",
  "iam_request_url":"aHR0cHM6Ly9zdHMuYW1hem9uYXdzLmNvbS8="
}`

	tokenJsonMissingUrlField = `{
  "iam_http_request_method":"POST",
  "iam_request_body":"QWN0aW9uPUdldENhbGxlcklkZW50aXR5JlZlcnNpb249MjAxMS0wNi0xNQ==",
  "iam_request_headers":"eyJBdXRob3JpemF0aW9uIjpbIkFXUzQtSE1BQy1TSEEyNTYgQ3JlZGVudGlhbD1mYWtlLzIwMjIwMzIyL3VzLWVhc3QtMS9zdHMvYXdzNF9yZXF1ZXN0LCBTaWduZWRIZWFkZXJzPWNvbnRlbnQtbGVuZ3RoO2NvbnRlbnQtdHlwZTtob3N0O3gtYW16LWRhdGU7eC1hbXotc2VjdXJpdHktdG9rZW4sIFNpZ25hdHVyZT1lZmMzMjBiOTcyZDA3YjM4YjY1ZWIyNDI1NjgwNWUwMzE0OWRhNTg2ZDgwNGY4YzYzNjRjZTk4ZGViZTA4MGIxIl0sIkNvbnRlbnQtTGVuZ3RoIjpbIjQzIl0sIkNvbnRlbnQtVHlwZSI6WyJhcHBsaWNhdGlvbi94LXd3dy1mb3JtLXVybGVuY29kZWQ7IGNoYXJzZXQ9dXRmLTgiXSwiVXNlci1BZ2VudCI6WyJhd3Mtc2RrLWdvLzEuNDIuMzQgKGdvMS4xNy41OyBkYXJ3aW47IGFtZDY0KSJdLCJYLUFtei1EYXRlIjpbIjIwMjIwMzIyVDIxMTEwM1oiXSwiWC1BbXotU2VjdXJpdHktVG9rZW4iOlsiZmFrZSJdfQ=="
}`
)
