// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package login

import (
	"encoding/json"
	"flag"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"

	iamauth "github.com/hashicorp/consul-awsauth"
	"github.com/hashicorp/consul/agent/consul/authmethod/awsauth"
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
		"Construct a bearer token and login to the AWS IAM auth method. This requires AWS credentials. "+
			"AWS credentials are automatically discovered from standard sources supported by the Go SDK for "+
			"AWS. Alternatively, explicit credentials can be passed using the -aws-acesss-key-id and "+
			"-aws-secret-access-key flags. [aws-iam only]")

	fs.BoolVar(&a.includeEntity, "aws-include-entity", false,
		"Include a signed request to get the IAM role or IAM user in the bearer token. [aws-iam only]")

	fs.StringVar(&a.stsEndpoint, "aws-sts-endpoint", "",
		"URL for AWS STS API calls. [aws-iam only]")

	fs.StringVar(&a.region, "aws-region", "",
		"Region for AWS API calls. If set, should match the region of -aws-sts-endpoint. "+
			"If not provided, the region will be discovered from standard sources, such as "+
			"the AWS_REGION environment variable. [aws-iam only]")

	fs.StringVar(&a.serverIDHeaderValue, "aws-server-id-header-value", "",
		"If set, an X-Consul-IAM-ServerID header is included in signed AWS API request(s) that form "+
			"the bearer token. This value must match the server-side configured value for the auth method "+
			"in order to login. This is optional and helps protect against replay attacks. [aws-iam only]")

	fs.StringVar(&a.accessKeyId, "aws-access-key-id", "",
		"AWS access key id to use. Requires -aws-secret-access-key if specified. [aws-iam only]")

	fs.StringVar(&a.secretAccessKey, "aws-secret-access-key", "",
		"AWS secret access key to use. Requires -aws-access-key-id if specified. [aws-iam only]")

	fs.StringVar(&a.sessionToken, "aws-session-token", "",
		"AWS session token to use. Requires -aws-access-key-id and -aws-secret-access-key if "+
			"specified. [aws-iam only]")
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

	if a.accessKeyId != "" {
		// Use creds from flags.
		cfg.Credentials = credentials.NewStaticCredentials(
			a.accessKeyId, a.secretAccessKey, a.sessionToken,
		)
	}

	// Session loads creds from standard sources (env vars, file, EC2 metadata, ...)
	sess, err := session.NewSessionWithOptions(session.Options{
		Config: cfg,
		// Allow loading from config files by default:
		//   ~/.aws/config or AWS_CONFIG_FILE
		//   ~/.aws/credentials or AWS_SHARED_CREDENTIALS_FILE
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		return "", err
	}
	if sess.Config.Region == nil || *sess.Config.Region == "" {
		return "", fmt.Errorf("AWS region not found")
	}
	if sess.Config.Credentials == nil {
		return "", fmt.Errorf("AWS credentials not found")
	}
	creds := sess.Config.Credentials

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
