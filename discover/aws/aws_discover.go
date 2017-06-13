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
)

type Config struct {
	Region          string
	TagKey          string
	TagValue        string
	AccessKeyID     string
	SecretAccessKey string
}

// Discover returns the ip addresses of all AWS instances in a region
// where TagKey == TagValue. If no region is provided the region of the
// instance is used.
func Discover(c *Config, l *log.Logger) ([]string, error) {
	if c == nil {
		return nil, fmt.Errorf("[ERR] discover-aws: Missing configuration")
	}

	region := c.Region
	if region == "" {
		l.Printf("[INFO] discover-aws: Looking up region")
		ec2meta := ec2metadata.New(session.New())
		identity, err := ec2meta.GetInstanceIdentityDocument()
		if err != nil {
			return nil, err
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
						AccessKeyID:     c.AccessKeyID,
						SecretAccessKey: c.SecretAccessKey,
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
				Name: aws.String("tag:" + c.TagKey),
				Values: []*string{
					aws.String(c.TagValue),
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}
	l.Printf("[INFO] discover-aws: Filter instances by %s=%s", c.TagKey, c.TagValue)

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
