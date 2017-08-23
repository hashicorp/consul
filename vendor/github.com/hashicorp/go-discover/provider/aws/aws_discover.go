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

	log.Printf("[DEBUG] discover-aws: Using region=%s tag_key=%s tag_value=%s", region, tagKey, tagValue)
	if accessKey == "" && secretKey == "" {
		log.Printf("[DEBUG] discover-aws: No static credentials")
		log.Printf("[DEBUG] discover-aws: Using environment variables, shared credentials or instance role")
	} else {
		log.Printf("[DEBUG] discover-aws: Static credentials provided")
	}

	if region == "" {
		l.Printf("[INFO] discover-aws: Region not provided. Looking up region in metadata...")
		ec2meta := ec2metadata.New(session.New())
		identity, err := ec2meta.GetInstanceIdentityDocument()
		if err != nil {
			return nil, fmt.Errorf("discover-aws: GetInstanceIdentityDocument failed: %s", err)
		}
		region = identity.Region
	}
	l.Printf("[INFO] discover-aws: Region is %s", region)

	l.Printf("[DEBUG] discover-aws: Creating session...")
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

	l.Printf("[INFO] discover-aws: Filter instances with %s=%s", tagKey, tagValue)
	resp, err := svc.DescribeInstances(&ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			&ec2.Filter{
				Name:   aws.String("tag:" + tagKey),
				Values: []*string{aws.String(tagValue)},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("discover-aws: DescribeInstancesInput failed: %s", err)
	}

	l.Printf("[DEBUG] discover-aws: Found %d reservations", len(resp.Reservations))
	var addrs []string
	for _, r := range resp.Reservations {
		l.Printf("[DEBUG] discover-aws: Reservation %s has %d instances", *r.ReservationId, len(r.Instances))
		for _, inst := range r.Instances {
			id := *inst.InstanceId
			l.Printf("[DEBUG] discover-aws: Found instance %s", id)

			// Terminated instances don't have the PrivateIpAddress field
			if inst.PrivateIpAddress == nil {
				l.Printf("[DEBUG] discover-aws: Instance %s has no private ip", id)
				continue
			}

			l.Printf("[INFO] discover-aws: Instance %s has private ip %s", id, *inst.PrivateIpAddress)
			addrs = append(addrs, *inst.PrivateIpAddress)
		}
	}

	l.Printf("[DEBUG] discover-aws: Found ip addresses: %v", addrs)
	return addrs, nil
}
