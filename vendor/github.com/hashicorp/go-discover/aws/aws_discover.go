// Package aws provides node discovery for Amazon AWS.
package aws

import (
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/defaults"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/hashicorp/go-discover/config"
)

// Discover returns the private ip addresses of all AWS instances in a
// region with a given tag key and value. If no region is provided the
// region of the instance is used.
//
// cfg contains the configuration in "key=val key=val ..." format. The
// values are URL encoded.
//
// The supported keys are:
//
//   region:            The AWS region
//   tag_key:           The tag key to filter on
//   tag_value:         The tag value to filter on
//   access_key_id:     The AWS access key to use
//   secret_access_key: The AWS secret access key to use
//
// Example:
//
//  region=eu-west-1 tag_key=consul tag_value=xxx access_key_id=xxx secret_access_key=xxx
//
func Discover(cfg string, l *log.Logger) ([]string, error) {
	m, err := config.Parse(cfg)
	if err != nil {
		return nil, fmt.Errorf("discover-aws: %s", err)
	}

	region := m["region"]
	tagKey := m["tag_key"]
	tagValue := m["tag_value"]
	accessKey := m["access_key_id"]
	secretKey := m["secret_access_key"]

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
