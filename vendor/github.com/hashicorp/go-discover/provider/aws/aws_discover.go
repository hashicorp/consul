// Package aws provides node discovery for Amazon AWS.
package aws

import (
	"fmt"
	"io/ioutil"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/defaults"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

type Provider struct{}

func (p *Provider) Help() string {
	return `Amazon AWS:

    provider:          "aws"
    region:            The AWS region. Default to region of instance.
    tag_key:           The tag key to filter on
    tag_value:         The tag value to filter on
    access_key_id:     The AWS access key to use
    secret_access_key: The AWS secret access key to use

    The only required IAM permission is 'ec2:DescribeInstances'. It is
    recommended you make a dedicated key used only for auto-joining.
`
}

func (p *Provider) Addrs(args map[string]string, l *log.Logger) ([]string, error) {
	if args["provider"] != "aws" {
		return nil, fmt.Errorf("discover-aws: invalid provider " + args["provider"])
	}

	if l == nil {
		l = log.New(ioutil.Discard, "", 0)
	}

	region := args["region"]
	tagKey := args["tag_key"]
	tagValue := args["tag_value"]
	accessKey := args["access_key_id"]
	secretKey := args["secret_access_key"]

	if region == "" {
		l.Printf("[INFO] discover-aws: Looking up region")
		ec2meta := ec2metadata.New(session.New())
		identity, err := ec2meta.GetInstanceIdentityDocument()
		if err != nil {
			return nil, fmt.Errorf("discover-aws: %s", err)
		}
		region = identity.Region
	}
	l.Printf("[INFO] discover-aws: Region is %s", region)

	svc := ec2.New(session.New(), &aws.Config{
		Region: &region,
		Credentials: credentials.NewChainCredentials(
			[]credentials.Provider{
				&credentials.StaticProvider{
					Value: credentials.Value{
						AccessKeyID:     accessKey,
						SecretAccessKey: secretKey,
					},
				},
				&credentials.EnvProvider{},
				&credentials.SharedCredentialsProvider{},
				defaults.RemoteCredProvider(*(defaults.Config()), defaults.Handlers()),
			}),
	})

	resp, err := svc.DescribeInstances(&ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("tag:" + tagKey),
				Values: []*string{aws.String(tagValue)},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("discover-aws: %s", err)
	}
	l.Printf("[INFO] discover-aws: Filter instances by %s=%s", tagKey, tagValue)

	var addrs []string
	for i := range resp.Reservations {
		for _, inst := range resp.Reservations[i].Instances {
			// Terminated instances don't have the PrivateIpAddress field
			if inst.PrivateIpAddress == nil {
				continue
			}
			l.Printf("[INFO] discover-aws: Found %x", *inst.PrivateIpAddress)
			addrs = append(addrs, *inst.PrivateIpAddress)
		}
	}
	return addrs, nil
}
