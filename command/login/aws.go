package login

import (
	"encoding/json"
	"flag"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/hashicorp/consul/agent/consul/authmethod/awsauth"
	"github.com/hashicorp/consul/internal/iamauth"
	"github.com/hashicorp/go-hclog"
)

type AWSLogin struct {
	autoBearerToken     bool
	includeEntity       bool
	stsEndpoint         string
	region              string
	serverIDHeaderValue string
	accessKeyId         string
	secretAccessKey     string
	sessionToken        string
}

func (a *AWSLogin) flags() *flag.FlagSet {
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	fs.BoolVar(&a.autoBearerToken, "aws-auto-bearer-token", false,
		"Automatically discover AWS credentials, construct the bearer token, and login. [aws-iam only]")

	fs.BoolVar(&a.includeEntity, "aws-include-entity", false,
		"Include a signed request to get the IAM role or IAM user in the bearer token. [aws-iam only]")

	fs.StringVar(&a.stsEndpoint, "aws-sts-endpoint", "",
		"URL for AWS STS API calls. [aws-iam only]")

	fs.StringVar(&a.region, "aws-region", "",
		"Region for AWS STS API calls. Should only be set if -aws-sts-endpoint is set. If set, should "+
			"match the region of -aws-sts-endpoint. [aws-iam only]")

	fs.StringVar(&a.serverIDHeaderValue, "aws-server-id-header-value", "",
		"If set, the X-Consul-IAM-ServerID header is included in the signed AWS API request(s) that form "+
			"the bearer token. This value must match the server-side configured value for the aws-iam auth method in order "+
			"to login. This is optional and helps protect against replay attacks. [aws-iam only]")

	fs.StringVar(&a.accessKeyId, "aws-access-key-id", "",
		"The AWS credential to use instead of automatically dsicovering credentials. [aws-iam only]")
	fs.StringVar(&a.secretAccessKey, "aws-secret-access-key", "",
		"The AWS credential to use instead of automatically dsicovering credentials. [aws-iam only]")
	fs.StringVar(&a.sessionToken, "aws-session-token", "",
		"The AWS credential to use instead of automatically dsicovering credentials. [aws-iam only]")
	return fs
}

// checkFlags validates flags for the aws-iam auth method.
func (a *AWSLogin) checkFlags() error {
	if !a.autoBearerToken {
		if a.includeEntity || a.stsEndpoint != "" || a.region != "" || a.serverIDHeaderValue != "" ||
			a.accessKeyId != "" || a.secretAccessKey != "" || a.sessionToken != "" {
			return fmt.Errorf("Missing '-aws-auto-bearer-token' flag")
		}
	}
	if a.accessKeyId != "" && a.secretAccessKey == "" {
		return fmt.Errorf("Missing '-aws-secret-access-key' flag")
	}
	if a.secretAccessKey != "" && a.accessKeyId == "" {
		return fmt.Errorf("Missing '-aws-access-key-id' flag")
	}
	if a.sessionToken != "" && (a.accessKeyId == "" || a.secretAccessKey == "") {
		return fmt.Errorf("Missing '-aws-access-key-id' and '-aws-secret-access-key' flags")
	}
	return nil
}

// createAWSBearerToken generates a bearer token string for the AWS IAM auth method.
// It will discover AWS credentials which are used to sign AWS API requests.
// Alternatively, static credentials can be passed as flags.
//
// The bearer token contains a signed sts:GetCallerIdentity request.
// If aws-include-entity is specified, a signed iam:GetRole or iam:GetUser request is
// also included. The AWS credentials are used to retrieve the current user's role
// or user name for the iam:GetRole or iam:GetUser request.
func (a *AWSLogin) createAWSBearerToken() (string, error) {
	cfg := aws.Config{
		Endpoint: aws.String(a.stsEndpoint),
		Region:   aws.String(a.region),
		// More detailed error message to help debug credential discovery.
		CredentialsChainVerboseErrors: aws.Bool(true),
	}

	var creds *credentials.Credentials
	if a.accessKeyId != "" {
		// Use creds from flags.
		creds = credentials.NewStaticCredentials(
			a.accessKeyId, a.secretAccessKey, a.sessionToken,
		)
	} else {
		// Session loads creds from standard sources (env vars, file, EC2 metadata, ...)
		sess, err := session.NewSessionWithOptions(session.Options{Config: cfg})
		if err != nil {
			return "", err
		}
		if sess.Config.Region == nil || *sess.Config.Region == "" {
			return "", fmt.Errorf("AWS region is required (-aws-region or AWS_REGION)")
		}
		if sess.Config.Credentials == nil {
			return "", fmt.Errorf("Failed to discover AWS credentials")
		}
		creds = sess.Config.Credentials
	}

	loginData, err := iamauth.GenerateLoginData(&iamauth.LoginInput{
		Creds:                  creds,
		IncludeIAMEntity:       a.includeEntity,
		STSEndpoint:            a.stsEndpoint,
		STSRegion:              a.region,
		Logger:                 hclog.New(nil),
		ServerIDHeaderValue:    a.serverIDHeaderValue,
		ServerIDHeaderName:     awsauth.IAMServerIDHeaderName,
		GetEntityMethodHeader:  awsauth.GetEntityMethodHeader,
		GetEntityURLHeader:     awsauth.GetEntityURLHeader,
		GetEntityHeadersHeader: awsauth.GetEntityHeadersHeader,
		GetEntityBodyHeader:    awsauth.GetEntityBodyHeader,
	})
	if err != nil {
		return "", err
	}

	loginDataJson, err := json.Marshal(loginData)
	if err != nil {
		return "", err
	}

	return string(loginDataJson), err
}
