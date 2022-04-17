package responses

import (
	"fmt"
	"strings"
)

// https://github.com/hashicorp/vault/blob/ba533d006f2244103648785ebfe8a9a9763d2b6e/builtin/credential/aws/path_login.go#L1722-L1744
type ParsedArn struct {
	Partition     string
	AccountNumber string
	Type          string
	Path          string
	FriendlyName  string
	SessionInfo   string
}

// https://github.com/hashicorp/vault/blob/ba533d006f2244103648785ebfe8a9a9763d2b6e/builtin/credential/aws/path_login.go#L1482-L1530
// However, instance profiles are not support in Consul.
func ParseArn(iamArn string) (*ParsedArn, error) {
	// iamArn should look like one of the following:
	// 1. arn:aws:iam::<account_id>:<entity_type>/<UserName>
	// 2. arn:aws:sts::<account_id>:assumed-role/<RoleName>/<RoleSessionName>
	// if we get something like 2, then we want to transform that back to what
	// most people would expect, which is arn:aws:iam::<account_id>:role/<RoleName>
	var entity ParsedArn
	fullParts := strings.Split(iamArn, ":")
	if len(fullParts) != 6 {
		return nil, fmt.Errorf("unrecognized arn: contains %d colon-separated parts, expected 6", len(fullParts))
	}
	if fullParts[0] != "arn" {
		return nil, fmt.Errorf("unrecognized arn: does not begin with \"arn:\"")
	}
	// normally aws, but could be aws-cn or aws-us-gov
	entity.Partition = fullParts[1]
	if entity.Partition == "" {
		return nil, fmt.Errorf("unrecognized arn: %q is missing the partition", iamArn)
	}
	if fullParts[2] != "iam" && fullParts[2] != "sts" {
		return nil, fmt.Errorf("unrecognized service: %v, not one of iam or sts", fullParts[2])
	}
	// fullParts[3] is the region, which doesn't matter for AWS IAM entities
	entity.AccountNumber = fullParts[4]
	if entity.AccountNumber == "" {
		return nil, fmt.Errorf("unrecognized arn: %q is missing the account number", iamArn)
	}
	// fullParts[5] would now be something like user/<UserName> or assumed-role/<RoleName>/<RoleSessionName>
	parts := strings.Split(fullParts[5], "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("unrecognized arn: %q contains fewer than 2 slash-separated parts", fullParts[5])
	}
	entity.Type = parts[0]
	entity.Path = strings.Join(parts[1:len(parts)-1], "/")
	entity.FriendlyName = parts[len(parts)-1]
	// now, entity.FriendlyName should either be <UserName> or <RoleName>
	switch entity.Type {
	case "assumed-role":
		// Check for three parts for assumed role ARNs
		if len(parts) < 3 {
			return nil, fmt.Errorf("unrecognized arn: %q contains fewer than 3 slash-separated parts", fullParts[5])
		}
		// Assumed roles don't have paths and have a slightly different format
		// parts[2] is <RoleSessionName>
		entity.Path = ""
		entity.FriendlyName = parts[1]
		entity.SessionInfo = parts[2]
	case "user":
	case "role":
	// case "instance-profile":
	default:
		return nil, fmt.Errorf("unrecognized principal type: %q", entity.Type)
	}

	if entity.FriendlyName == "" {
		return nil, fmt.Errorf("unrecognized arn: %q is missing the resource name", iamArn)
	}

	return &entity, nil
}

// CanonicalArn returns the canonical ARN for referring to an IAM entity
func (p *ParsedArn) CanonicalArn() string {
	entityType := p.Type
	// canonicalize "assumed-role" into "role"
	if entityType == "assumed-role" {
		entityType = "role"
	}
	// Annoyingly, the assumed-role entity type doesn't have the Path of the role which was assumed
	// So, we "canonicalize" it by just completely dropping the path. The other option would be to
	// make an AWS API call to look up the role by FriendlyName, which introduces more complexity to
	// code and test, and it also breaks backwards compatibility in an area where we would really want
	// it
	return fmt.Sprintf("arn:%s:iam::%s:%s/%s", p.Partition, p.AccountNumber, entityType, p.FriendlyName)
}
