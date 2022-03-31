package iamauth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"

	"github.com/hashicorp/consul/lib/stringslice"
)

const (
	amzHeaderPrefix    = "X-Amz-"
	defaultIAMEndpoint = "https://iam.amazonaws.com"
	defaultSTSEndpoint = "https://sts.amazonaws.com"
)

var defaultAllowedSTSRequestHeaders = []string{
	"X-Amz-Algorithm",
	"X-Amz-Content-Sha256",
	"X-Amz-Credential",
	"X-Amz-Date",
	"X-Amz-Security-Token",
	"X-Amz-Signature",
	"X-Amz-SignedHeaders",
}

// BearerToken is a login "token" for an IAM auth method. It is a signed
// sts:GetCallerIdentity request in JSON format. Optionally, it can include a
// signed embedded iam:GetRole or iam:GetUser request in the headers.
type BearerToken struct {
	config *Config

	getCallerIdentityMethod string
	getCallerIdentityURL    string
	getCallerIdentityHeader http.Header
	getCallerIdentityBody   string

	getIAMEntityMethod string
	getIAMEntityURL    string
	getIAMEntityHeader http.Header
	getIAMEntityBody   string

	entityRequestType       string
	parsedCallerIdentityURL *url.URL
	parsedIAMEntityURL      *url.URL
}

var _ json.Unmarshaler = (*BearerToken)(nil)

func NewBearerToken(loginToken string, config *Config) (*BearerToken, error) {
	token := &BearerToken{config: config}
	if err := json.Unmarshal([]byte(loginToken), &token); err != nil {
		return nil, fmt.Errorf("invalid token: %s", err)
	}

	if err := token.validate(); err != nil {
		return nil, err
	}

	if config.EnableIAMEntityDetails {
		method, err := token.getHeader(token.config.GetEntityMethodHeader)
		if err != nil {
			return nil, err
		}

		rawUrl, err := token.getHeader(token.config.GetEntityURLHeader)
		if err != nil {
			return nil, err
		}

		headerJson, err := token.getHeader(token.config.GetEntityHeadersHeader)
		if err != nil {
			return nil, err
		}

		var header http.Header
		if err := json.Unmarshal([]byte(headerJson), &header); err != nil {
			return nil, err
		}

		body, err := token.getHeader(token.config.GetEntityBodyHeader)
		if err != nil {
			return nil, err
		}

		parsedUrl, err := parseUrl(rawUrl)
		if err != nil {
			return nil, err
		}

		token.getIAMEntityMethod = method
		token.getIAMEntityBody = body
		token.getIAMEntityURL = rawUrl
		token.getIAMEntityHeader = header
		token.parsedIAMEntityURL = parsedUrl

		reqType, err := token.validateIAMEntityBody()
		if err != nil {
			return nil, err
		}
		token.entityRequestType = reqType
	}
	return token, nil
}

// https://github.com/hashicorp/vault/blob/b17e3256dde937a6248c9a2fa56206aac93d07de/builtin/credential/aws/path_login.go#L1178
func (t *BearerToken) validate() error {
	if t.getCallerIdentityMethod != "POST" {
		return fmt.Errorf("iam_http_request_method must be POST")
	}
	if err := t.validateGetCallerIdentityBody(); err != nil {
		return err
	}
	if err := t.validateAllowedSTSHeaderValues(); err != nil {
		return err
	}
	return nil
}

// https://github.com/hashicorp/vault/blob/b17e3256dde937a6248c9a2fa56206aac93d07de/builtin/credential/aws/path_login.go#L1439
func (t *BearerToken) validateGetCallerIdentityBody() error {
	allowedValues := url.Values{
		"Action": []string{"GetCallerIdentity"},
		// Will assume for now that future versions don't change
		// the semantics
		"Version": nil, // any value is allowed
	}
	if _, err := parseRequestBody(t.getCallerIdentityBody, allowedValues); err != nil {
		return fmt.Errorf("iam_request_body error: %s", err)
	}

	return nil
}

func (t *BearerToken) validateIAMEntityBody() (string, error) {
	allowedValues := url.Values{
		"Action":   []string{"GetRole", "GetUser"},
		"RoleName": nil, // any value is allowed
		"UserName": nil,
		"Version":  nil,
	}
	body, err := parseRequestBody(t.getIAMEntityBody, allowedValues)
	if err != nil {
		return "", fmt.Errorf("iam_request_headers[%s] error: %s", t.config.GetEntityBodyHeader, err)
	}

	// Disallow GetRole+UserName and GetUser+RoleName.
	action := body["Action"][0]
	_, hasRoleName := body["RoleName"]
	_, hasUserName := body["UserName"]
	if action == "GetUser" && hasUserName && !hasRoleName {
		return action, nil
	} else if action == "GetRole" && hasRoleName && !hasUserName {
		return action, nil
	}
	return "", fmt.Errorf("iam_request_headers[%q] error: invalid request body %q", t.config.GetEntityBodyHeader, t.getIAMEntityBody)
}

// parseRequestBody parses the AWS STS or IAM request body, such as 'Action=GetRole&RoleName=my-role'.
// It returns the parsed values, or an error if there are unexpected fields based on allowedValues.
//
// A key-value pair in the body is allowed if:
//  - It is a single value (i.e. no bodies like 'Action=1&Action=2')
//  - allowedValues[key] is an empty slice or nil (any value is allowed for the key)
//  - allowedValues[key] is non-empty and contains the exact value
// This always requires an 'Action' field is present and non-empty.
func parseRequestBody(body string, allowedValues url.Values) (url.Values, error) {
	qs, err := url.ParseQuery(body)
	if err != nil {
		return nil, err
	}

	// Action field is always required.
	if _, ok := qs["Action"]; !ok || len(qs["Action"]) == 0 || qs["Action"][0] == "" {
		return nil, fmt.Errorf(`missing field "Action"`)
	}

	// Ensure the body does not have extra fields and each
	// field in the body matches the allowed values.
	for k, v := range qs {
		exp, ok := allowedValues[k]
		if k != "Action" && !ok {
			return nil, fmt.Errorf("unexpected field %q", k)
		}

		if len(exp) == 0 {
			// empty indicates any value is okay
			continue
		} else if len(v) != 1 || !stringslice.Contains(exp, v[0]) {
			return nil, fmt.Errorf("unexpected value %s=%v", k, v)
		}
	}

	return qs, nil
}

// https://github.com/hashicorp/vault/blob/861454e0ed1390d67ddaf1a53c1798e5e291728c/builtin/credential/aws/path_config_client.go#L349
func (t *BearerToken) validateAllowedSTSHeaderValues() error {
	for k := range t.getCallerIdentityHeader {
		h := textproto.CanonicalMIMEHeaderKey(k)
		if strings.HasPrefix(h, amzHeaderPrefix) &&
			!stringslice.Contains(defaultAllowedSTSRequestHeaders, h) &&
			!stringslice.Contains(t.config.AllowedSTSHeaderValues, h) {
			return fmt.Errorf("invalid request header: %s", h)
		}
	}
	return nil
}

// UnmarshalJSON unmarshals the bearer token details which contains an HTTP
// request (a signed sts:GetCallerIdentity request).
func (t *BearerToken) UnmarshalJSON(data []byte) error {
	var rawData struct {
		Method        string `json:"iam_http_request_method"`
		UrlBase64     string `json:"iam_request_url"`
		HeadersBase64 string `json:"iam_request_headers"`
		BodyBase64    string `json:"iam_request_body"`
	}

	if err := json.Unmarshal(data, &rawData); err != nil {
		return err
	}

	rawUrl, err := base64.StdEncoding.DecodeString(rawData.UrlBase64)
	if err != nil {
		return err
	}

	headersJson, err := base64.StdEncoding.DecodeString(rawData.HeadersBase64)
	if err != nil {
		return err
	}

	var headers http.Header
	// This is a JSON-string in JSON
	if err := json.Unmarshal(headersJson, &headers); err != nil {
		return err
	}

	body, err := base64.StdEncoding.DecodeString(rawData.BodyBase64)
	if err != nil {
		return err
	}

	t.getCallerIdentityMethod = rawData.Method
	t.getCallerIdentityBody = string(body)
	t.getCallerIdentityHeader = headers
	t.getCallerIdentityURL = string(rawUrl)

	parsedUrl, err := parseUrl(t.getCallerIdentityURL)
	if err != nil {
		return err
	}
	t.parsedCallerIdentityURL = parsedUrl
	return nil
}

func parseUrl(s string) (*url.URL, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, err
	}
	// url.Parse doesn't error on empty string
	if u == nil || u.Scheme == "" || u.Host == "" || u.Path == "" {
		return nil, fmt.Errorf("url is invalid: %q", s)
	}
	return u, nil
}

// GetCallerIdentityRequest returns the sts:GetCallerIdentity request decoded
// from the bearer token.
func (t *BearerToken) GetCallerIdentityRequest() (*http.Request, error) {
	// NOTE: We need to ensure we're calling STS, instead of acting as an unintended network proxy
	// The protection against this is that this method will only call the endpoint specified in the
	// client config (defaulting to sts.amazonaws.com), so it would require an admin to override
	// the endpoint to talk to alternate web addresses
	endpoint := defaultSTSEndpoint
	if t.config.STSEndpoint != "" {
		endpoint = t.config.STSEndpoint
	}

	return buildHttpRequest(
		t.getCallerIdentityMethod,
		endpoint,
		t.parsedCallerIdentityURL,
		t.getCallerIdentityBody,
		t.getCallerIdentityHeader,
	)
}

// GetEntityRequest returns the iam:GetUser or iam:GetRole request from the request details,
// if present, embedded in the headers of the sts:GetCallerIdentity request.
func (t *BearerToken) GetEntityRequest() (*http.Request, error) {
	endpoint := defaultIAMEndpoint
	if t.config.IAMEndpoint != "" {
		endpoint = t.config.IAMEndpoint
	}

	return buildHttpRequest(
		t.getIAMEntityMethod,
		endpoint,
		t.parsedIAMEntityURL,
		t.getIAMEntityBody,
		t.getIAMEntityHeader,
	)
}

// getHeader returns the header from s.GetCallerIdentityHeader, or an error if
// the header is not found or is not a single value.
func (t *BearerToken) getHeader(name string) (string, error) {
	values := t.getCallerIdentityHeader.Values(name)
	if len(values) == 0 {
		return "", fmt.Errorf("missing header %q", name)
	}
	if len(values) != 1 {
		return "", fmt.Errorf("invalid value for header %q (expected 1 item)", name)
	}
	return values[0], nil
}

// buildHttpRequest returns an HTTP request from the given details.
// This supports sending to a custom endpoint, but always preserves the
// Host header and URI path, which are signed and cannot be modified.
// There's a deeper explanation of this in the Vault source code.
// https://github.com/hashicorp/vault/blob/b17e3256dde937a6248c9a2fa56206aac93d07de/builtin/credential/aws/path_login.go#L1569
func buildHttpRequest(method, endpoint string, parsedUrl *url.URL, body string, headers http.Header) (*http.Request, error) {
	targetUrl := fmt.Sprintf("%s%s", endpoint, parsedUrl.RequestURI())
	request, err := http.NewRequest(method, targetUrl, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Host = parsedUrl.Host
	for k, vals := range headers {
		for _, val := range vals {
			request.Header.Add(k, val)
		}
	}
	return request, nil
}
