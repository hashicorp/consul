package aws

import (
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/defaults"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

// Discover returns the ip addresses of all AWS instances in a region
// where tag_key == tag_value. If no region is provided the region of the
// instance is used.
//
// cfg supports the following fields:
//
//   "region":            the AWS region
//   "tag_key":           the tag key to filter on
//   "tag_value":         the tag value to filter on
//   "access_key_id":     the AWS access key to use
//   "secret_access_key": the AWS secret access key to use
//
func Discover(cfg map[string]string, l *log.Logger) ([]string, error) {
	region := cfg["region"]
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

	tagKey, tagValue := cfg["tag_key"], cfg["tag_value"]
	accessKey, secretKey := cfg["access_key_id"], cfg["secret_access_key"]

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
		return nil, err
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
