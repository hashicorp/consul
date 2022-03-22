package iamauth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/hashicorp/consul/internal/iamauth/responses"
	"github.com/hashicorp/go-hclog"
)

type LoginInput struct {
	Creds            *credentials.Credentials
	IncludeIAMEntity bool
	STSEndpoint      string
	STSRegion        string

	Logger hclog.Logger

	ServerIDHeaderValue string
	// Customizable header names
	ServerIDHeaderName     string
	GetEntityMethodHeader  string
	GetEntityURLHeader     string
	GetEntityHeadersHeader string
	GetEntityBodyHeader    string
}

// GenerateLoginData populates the necessary data to send for the bearer token.
// https://github.com/hashicorp/go-secure-stdlib/blob/main/awsutil/generate_credentials.go#L232-L301
func GenerateLoginData(in *LoginInput) (map[string]interface{}, error) {
	cfg := aws.Config{
		Credentials: in.Creds,
		Region:      aws.String(in.STSRegion),
	}
	if in.STSEndpoint != "" {
		cfg.Endpoint = aws.String(in.STSEndpoint)
	} else {
		cfg.EndpointResolver = endpoints.ResolverFunc(stsSigningResolver)
	}

	stsSession, err := session.NewSessionWithOptions(session.Options{Config: cfg})
	if err != nil {
		return nil, err
	}

	svc := sts.New(stsSession)
	stsRequest, _ := svc.GetCallerIdentityRequest(nil)

	// Include the iam:GetRole or iam:GetUser request in headers.
	if in.IncludeIAMEntity {
		entityRequest, err := formatSignedEntityRequest(svc, in)
		if err != nil {
			return nil, err
		}

		headersJson, err := json.Marshal(entityRequest.HTTPRequest.Header)
		if err != nil {
			return nil, err
		}
		requestBody, err := ioutil.ReadAll(entityRequest.HTTPRequest.Body)
		if err != nil {
			return nil, err
		}

		stsRequest.HTTPRequest.Header.Add(in.GetEntityMethodHeader, entityRequest.HTTPRequest.Method)
		stsRequest.HTTPRequest.Header.Add(in.GetEntityURLHeader, entityRequest.HTTPRequest.URL.String())
		stsRequest.HTTPRequest.Header.Add(in.GetEntityHeadersHeader, string(headersJson))
		stsRequest.HTTPRequest.Header.Add(in.GetEntityBodyHeader, string(requestBody))
	}

	// Inject the required auth header value, if supplied, and then sign the request including that header
	if in.ServerIDHeaderValue != "" {
		stsRequest.HTTPRequest.Header.Add(in.ServerIDHeaderName, in.ServerIDHeaderValue)
	}

	stsRequest.Sign()

	// Now extract out the relevant parts of the request
	headersJson, err := json.Marshal(stsRequest.HTTPRequest.Header)
	if err != nil {
		return nil, err
	}
	requestBody, err := ioutil.ReadAll(stsRequest.HTTPRequest.Body)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"iam_http_request_method": stsRequest.HTTPRequest.Method,
		"iam_request_url":         base64.StdEncoding.EncodeToString([]byte(stsRequest.HTTPRequest.URL.String())),
		"iam_request_headers":     base64.StdEncoding.EncodeToString(headersJson),
		"iam_request_body":        base64.StdEncoding.EncodeToString(requestBody),
	}, nil
}

// STS is a really weird service that used to only have global endpoints but now has regional endpoints as well.
// For backwards compatibility, even if you request a region other than us-east-1, it'll still sign for us-east-1.
// See, e.g., https://docs.aws.amazon.com/IAM/latest/UserGuide/id_credentials_temp_enable-regions.html#id_credentials_temp_enable-regions_writing_code
// So we have to shim in this EndpointResolver to force it to sign for the right region
func stsSigningResolver(service, region string, optFns ...func(*endpoints.Options)) (endpoints.ResolvedEndpoint, error) {
	defaultEndpoint, err := endpoints.DefaultResolver().EndpointFor(service, region, optFns...)
	if err != nil {
		return defaultEndpoint, err
	}
	defaultEndpoint.SigningRegion = region
	return defaultEndpoint, nil
}

func formatSignedEntityRequest(svc *sts.STS, in *LoginInput) (*request.Request, error) {
	// We need to retrieve the IAM user or role for the iam:GetRole or iam:GetUser request.
	// GetCallerIdentity returns this and requires no permissions.
	resp, err := svc.GetCallerIdentity(nil)
	if err != nil {
		return nil, err
	}

	arn, err := responses.ParseArn(*resp.Arn)
	if err != nil {
		return nil, err
	}

	iamSession, err := session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Credentials: svc.Config.Credentials,
		},
	})
	if err != nil {
		return nil, err
	}
	iamSvc := iam.New(iamSession)

	var req *request.Request
	switch arn.Type {
	case "role", "assumed-role":
		req, _ = iamSvc.GetRoleRequest(&iam.GetRoleInput{RoleName: &arn.FriendlyName})
	case "user":
		req, _ = iamSvc.GetUserRequest(&iam.GetUserInput{UserName: &arn.FriendlyName})
	default:
		return nil, fmt.Errorf("entity %s is not an IAM role or IAM user", arn.Type)
	}

	// Inject the required auth header value, if supplied, and then sign the request including that header
	if in.ServerIDHeaderValue != "" {
		req.HTTPRequest.Header.Add(in.ServerIDHeaderName, in.ServerIDHeaderValue)
	}

	req.Sign()
	return req, nil
}
