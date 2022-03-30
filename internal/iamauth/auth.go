package iamauth

import (
	"context"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/consul/internal/iamauth/responses"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/lib/stringslice"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-retryablehttp"
)

const (
	// Retry configuration
	retryWaitMin = 500 * time.Millisecond
	retryWaitMax = 30 * time.Second
)

type Authenticator struct {
	config *Config
	logger hclog.Logger
}

type IdentityDetails struct {
	EntityName string
	EntityId   string
	AccountId  string

	EntityPath string
	EntityTags map[string]string
}

func NewAuthenticator(config *Config, logger hclog.Logger) (*Authenticator, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &Authenticator{
		config: config,
		logger: logger,
	}, nil
}

// ValidateLogin determines if the identity in the loginToken is permitted to login.
// If so, it returns details about the identity. Otherwise, an error is returned.
func (a *Authenticator) ValidateLogin(ctx context.Context, loginToken string) (*IdentityDetails, error) {
	token, err := NewBearerToken(loginToken, a.config)
	if err != nil {
		return nil, err
	}

	req, err := token.GetCallerIdentityRequest()
	if err != nil {
		return nil, err
	}

	if a.config.ServerIDHeaderValue != "" {
		err := validateHeaderValue(req.Header, a.config.ServerIDHeaderName, a.config.ServerIDHeaderValue)
		if err != nil {
			return nil, err
		}
	}

	callerIdentity, err := a.submitCallerIdentityRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	a.logger.Debug("iamauth login attempt", "arn", callerIdentity.Arn)

	entity, err := responses.ParseArn(callerIdentity.Arn)
	if err != nil {
		return nil, err
	}

	identityDetails := &IdentityDetails{
		EntityName: entity.FriendlyName,
		// This could either be a "userID:SessionID" (in the case of an assumed role) or just a "userID"
		// (in the case of an IAM user).
		EntityId:  strings.Split(callerIdentity.UserId, ":")[0],
		AccountId: callerIdentity.Account,
	}
	clientArn := entity.CanonicalArn()

	// Fetch the IAM Role or IAM User, if configured.
	// This requires the token to contain a signed iam:GetRole or iam:GetUser request.
	if a.config.EnableIAMEntityDetails {
		iamReq, err := token.GetEntityRequest()
		if err != nil {
			return nil, err
		}

		if a.config.ServerIDHeaderValue != "" {
			err := validateHeaderValue(iamReq.Header, a.config.ServerIDHeaderName, a.config.ServerIDHeaderValue)
			if err != nil {
				return nil, err
			}
		}

		iamEntityDetails, err := a.submitGetIAMEntityRequest(ctx, iamReq, token.entityRequestType)
		if err != nil {
			return nil, err
		}

		// Only the CallerIdentity response is a guarantee of the client's identity.
		// The role/user details must have a unique id match to the CallerIdentity before use.
		if iamEntityDetails.EntityId() != identityDetails.EntityId {
			return nil, fmt.Errorf("unique id mismatch in login token")
		}

		// Use the full ARN with path from the Role/User details
		clientArn = iamEntityDetails.EntityArn()
		identityDetails.EntityPath = iamEntityDetails.EntityPath()
		identityDetails.EntityTags = iamEntityDetails.EntityTags()
	}

	if err := a.validateIdentity(clientArn); err != nil {
		return nil, err
	}
	return identityDetails, nil
}

// https://github.com/hashicorp/vault/blob/ba533d006f2244103648785ebfe8a9a9763d2b6e/builtin/credential/aws/path_login.go#L1321-L1361
func (a *Authenticator) validateIdentity(clientArn string) error {
	if stringslice.Contains(a.config.BoundIAMPrincipalARNs, clientArn) {
		// Matches one of BoundIAMPrincipalARNs, so it is trusted
		return nil
	}
	if a.config.EnableIAMEntityDetails {
		for _, principalArn := range a.config.BoundIAMPrincipalARNs {
			if strings.HasSuffix(principalArn, "*") && lib.GlobbedStringsMatch(principalArn, clientArn) {
				// Wildcard match, so it is trusted
				return nil
			}
		}
	}
	return fmt.Errorf("IAM principal %s is not trusted", clientArn)
}

func (a *Authenticator) submitCallerIdentityRequest(ctx context.Context, req *http.Request) (*responses.GetCallerIdentityResult, error) {
	responseBody, err := a.submitRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	callerIdentityResponse, err := parseGetCallerIdentityResponse(responseBody)
	if err != nil {
		return nil, fmt.Errorf("error parsing STS response")
	}

	if n := len(callerIdentityResponse.GetCallerIdentityResult); n != 1 {
		return nil, fmt.Errorf("received %d identities in STS response but expected 1", n)
	}
	return &callerIdentityResponse.GetCallerIdentityResult[0], nil
}

func (a *Authenticator) submitGetIAMEntityRequest(ctx context.Context, req *http.Request, reqType string) (responses.IAMEntity, error) {
	responseBody, err := a.submitRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	iamResponse, err := parseGetIAMEntityResponse(responseBody, reqType)
	if err != nil {
		return nil, fmt.Errorf("error parsing IAM response: %s", err)
	}
	return iamResponse, nil

}

// https://github.com/hashicorp/vault/blob/b17e3256dde937a6248c9a2fa56206aac93d07de/builtin/credential/aws/path_login.go#L1636
func (a *Authenticator) submitRequest(ctx context.Context, req *http.Request) (string, error) {
	retryableReq, err := retryablehttp.FromRequest(req)
	if err != nil {
		return "", err
	}
	retryableReq = retryableReq.WithContext(ctx)
	client := cleanhttp.DefaultClient()
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	retryingClient := &retryablehttp.Client{
		HTTPClient:   client,
		RetryWaitMin: retryWaitMin,
		RetryWaitMax: retryWaitMax,
		RetryMax:     a.config.MaxRetries,
		CheckRetry:   retryablehttp.DefaultRetryPolicy,
		Backoff:      retryablehttp.DefaultBackoff,
	}

	response, err := retryingClient.Do(retryableReq)
	if err != nil {
		return "", fmt.Errorf("error making request: %w", err)
	}
	if response != nil {
		defer response.Body.Close()
	}
	// Validate that the response type is XML
	if ct := response.Header.Get("Content-Type"); ct != "text/xml" {
		return "", fmt.Errorf("response body is invalid")
	}

	// we check for status code afterwards to also print out response body
	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	if response.StatusCode != 200 {
		return "", fmt.Errorf("received error code %d: %s", response.StatusCode, string(responseBody))
	}
	return string(responseBody), nil

}

// https://github.com/hashicorp/vault/blob/ba533d006f2244103648785ebfe8a9a9763d2b6e/builtin/credential/aws/path_login.go#L1625-L1634
func parseGetCallerIdentityResponse(response string) (responses.GetCallerIdentityResponse, error) {
	result := responses.GetCallerIdentityResponse{}
	response = strings.TrimSpace(response)
	if !strings.HasPrefix(response, "<GetCallerIdentityResponse") && !strings.HasPrefix(response, "<?xml") {
		return result, fmt.Errorf("body of GetCallerIdentity is invalid")
	}
	decoder := xml.NewDecoder(strings.NewReader(response))
	err := decoder.Decode(&result)
	return result, err
}

func parseGetIAMEntityResponse(response string, reqType string) (responses.IAMEntity, error) {
	if !strings.HasPrefix(response, "<GetRoleResponse") &&
		!strings.HasPrefix(response, "<GetUserResponse") &&
		!strings.HasPrefix(response, "<?xml") {
		return nil, fmt.Errorf("body of GetRole or GetUser is invalid")
	}

	decoder := xml.NewDecoder(strings.NewReader(response))

	switch reqType {
	case "GetRole":
		result := &responses.GetRoleResponse{}
		err := decoder.Decode(&result)
		if err != nil {
			return nil, err
		}
		if n := len(result.GetRoleResult); n != 1 {
			return nil, fmt.Errorf("received %d identities in GetRole response but expected 1", n)
		}
		return &result.GetRoleResult[0].Role, nil
	case "GetUser":
		result := &responses.GetUserResponse{}
		err := decoder.Decode(&result)
		if err != nil {
			return nil, err
		}
		if n := len(result.GetUserResult); n != 1 {
			return nil, fmt.Errorf("received %d identities in GetUser response but expected 1", n)
		}
		return &result.GetUserResult[0].User, nil
	}
	return nil, fmt.Errorf("invalid %s request: %s", reqType, response)
}

// https://github.com/hashicorp/vault/blob/b17e3256dde937a6248c9a2fa56206aac93d07de/builtin/credential/aws/path_login.go#L1532
func validateHeaderValue(headers http.Header, headerName string, requiredHeaderValue string) error {
	providedValue := ""
	for k, v := range headers {
		if strings.EqualFold(headerName, k) {
			providedValue = strings.Join(v, ",")
			break
		}
	}
	if providedValue == "" {
		return fmt.Errorf("missing header %q", headerName)
	}

	// NOT doing a constant time compare here since the value is NOT intended to be secret
	if providedValue != requiredHeaderValue {
		return fmt.Errorf("expected %q but got %q", requiredHeaderValue, providedValue)
	}

	if authzHeaders, ok := headers["Authorization"]; ok {
		// authzHeader looks like AWS4-HMAC-SHA256 Credential=AKI..., SignedHeaders=host;x-amz-date;x-vault-awsiam-id, Signature=...
		// We need to extract out the SignedHeaders
		re := regexp.MustCompile(".*SignedHeaders=([^,]+)")
		authzHeader := strings.Join(authzHeaders, ",")
		matches := re.FindSubmatch([]byte(authzHeader))
		if len(matches) < 1 {
			return fmt.Errorf("server id header wasn't signed")
		}
		if len(matches) > 2 {
			return fmt.Errorf("found multiple SignedHeaders components")
		}
		signedHeaders := string(matches[1])
		return ensureHeaderIsSigned(signedHeaders, headerName)
	}
	// NOTE: If we support GET requests, then we need to parse the X-Amz-SignedHeaders
	// argument out of the query string and search in there for the header value
	return fmt.Errorf("missing Authorization header")
}

func ensureHeaderIsSigned(signedHeaders, headerToSign string) error {
	// Not doing a constant time compare here, the values aren't secret
	for _, header := range strings.Split(signedHeaders, ";") {
		if header == strings.ToLower(headerToSign) {
			return nil
		}
	}
	return fmt.Errorf("header wasn't signed")
}
