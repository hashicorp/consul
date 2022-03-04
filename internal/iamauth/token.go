package iamauth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"

	"github.com/hashicorp/go-secure-stdlib/strutil"
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

	iamEntityType           IAMEntityType
	parsedCallerIdentityURL *url.URL
	parsedIAMEntityURL      *url.URL
}

var _ json.Unmarshaler = (*BearerToken)(nil)

type IAMEntityType string

const (
	IAMEntityTypeNone IAMEntityType = ""
	IAMEntityTypeRole               = "role"
	IAMEntityTypeUser               = "user"
)

func NewBearerToken(loginToken string, config *Config) (*BearerToken, error) {
	token := &BearerToken{config: config}
	if err := json.Unmarshal([]byte(loginToken), &token); err != nil {
		return nil, err
	}

	if err := token.validate(); err != nil {
		return nil, err
	}

	if config.EnableIAMEntityDetails {
		method, err := token.getHeader(token.config.GetEntityMethodHeader)
		if err != nil {
			return nil, err
		}

		url, err := token.getHeader(token.config.GetEntityURLHeader)
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

		parsedUrl, err := token.parsedCallerIdentityURL.Parse(url)
		if err != nil {
			return nil, err
		}

		token.getIAMEntityMethod = method
		token.getIAMEntityBody = body
		token.getIAMEntityURL = url
		token.getIAMEntityHeader = header
		token.parsedIAMEntityURL = parsedUrl

		entityType, err := token.validateIAMEntityBody()
		if err != nil {
			return nil, err
		}
		token.iamEntityType = entityType

		// TODO:
		// if err := t.validateServerIDHeader(); err != nil {
		// 	return err
		// }
		// if err := t.validateAllowedSTSHeaderValues(); err != nil {
		// 	return err
		// }

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
	qs, err := url.ParseQuery(t.getCallerIdentityBody)
	if err != nil {
		return err
	}
	for k, v := range qs {
		switch k {
		case "Action":
			if len(v) != 1 || v[0] != "GetCallerIdentity" {
				return fmt.Errorf("iam_request_body must have 'Action=GetCallerIdentity'")
			}
		case "Version":
		// Will assume for now that future versions don't change
		// the semantics
		default:
			// Not expecting any other values
			return fmt.Errorf("iam_request_body contains unexpected values")
		}
	}
	return nil
}

func (t *BearerToken) validateIAMEntityBody() (IAMEntityType, error) {
	qs, err := url.ParseQuery(t.getIAMEntityBody)
	if err != nil {
		return IAMEntityTypeNone, err
	}
	action := ""
	nameType := ""
loop:
	for k, v := range qs {
		switch k {
		case "Action":
			// Check for Action=GetRole or Action=GetUser, but allow nothing else.
			if len(v) == 1 {
				switch v[0] {
				case "GetRole", "GetUser":
					action = v[0]
					continue loop
				}
			}
			// invalid body
			action = ""
			break loop
		case "RoleName", "UserName":
			nameType = k
		case "Version":
		// Will assume for now that future versions don't change
		// the semantics
		default:
			// Not expecting any other values
			return IAMEntityTypeNone, fmt.Errorf("iam_request_headers[%q] contains unexpected value", t.config.GetEntityBodyHeader)
		}
	}
	if action == "GetUser" && nameType == "UserName" {
		return IAMEntityTypeUser, nil
	}
	if action == "GetRole" && nameType == "RoleName" {
		return IAMEntityTypeRole, nil
	}
	return IAMEntityTypeNone, fmt.Errorf("iam_request_headers[%q] contains unexpected value", t.config.GetEntityBodyHeader)
}

// https://github.com/hashicorp/vault/blob/861454e0ed1390d67ddaf1a53c1798e5e291728c/builtin/credential/aws/path_config_client.go#L349
func (t *BearerToken) validateAllowedSTSHeaderValues() error {
	for k := range t.getCallerIdentityHeader {
		h := textproto.CanonicalMIMEHeaderKey(k)
		if strings.HasPrefix(h, amzHeaderPrefix) &&
			!strutil.StrListContains(defaultAllowedSTSRequestHeaders, h) &&
			!strutil.StrListContains(t.config.AllowedSTSHeaderValues, h) {
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

	parsedUrl, err := url.Parse(t.getCallerIdentityURL)
	if err != nil {
		return err
	}
	t.parsedCallerIdentityURL = parsedUrl
	return nil
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
