package agent

import (
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/defaults"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

// discoverEc2Hosts searches an AWS region, returning a list of instance ips
// where EC2TagKey = EC2TagValue
func (c *Config) discoverEc2Hosts(logger *log.Logger) ([]string, error) {
	config := c.RetryJoinEC2

	ec2meta := ec2metadata.New(session.New())
	if config.Region == "" {
		logger.Printf("[INFO] agent: No EC2 region provided, querying instance metadata endpoint...")
		identity, err := ec2meta.GetInstanceIdentityDocument()
		if err != nil {
			return nil, err
		}
		config.Region = identity.Region
	}

	awsConfig := &aws.Config{
		Region: &config.Region,
		Credentials: credentials.NewChainCredentials(
			[]credentials.Provider{
				&credentials.StaticProvider{
					Value: credentials.Value{
						AccessKeyID:     config.AccessKeyID,
						SecretAccessKey: config.SecretAccessKey,
					},
				},
				&credentials.EnvProvider{},
				&credentials.SharedCredentialsProvider{},
				defaults.RemoteCredProvider(*(defaults.Config()), defaults.Handlers()),
			}),
	}

	svc := ec2.New(session.New(), awsConfig)

	resp, err := svc.DescribeInstances(&ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("tag:" + config.TagKey),
				Values: []*string{
					aws.String(config.TagValue),
				},
			},
		},
	})

	if err != nil {
		return nil, err
	}

	var servers []string
	for i := range resp.Reservations {
		for _, instance := range resp.Reservations[i].Instances {
			// Terminated instances don't have the PrivateIpAddress field
			if instance.PrivateIpAddress != nil {
				servers = append(servers, *instance.PrivateIpAddress)
			}
		}
	}

	return servers, nil
}
