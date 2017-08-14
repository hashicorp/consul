// +build !ent

package command

import (
	consulapi "github.com/hashicorp/consul/api"
)

// getSegmentMembers returns an empty list since network segments are not
// supported in OSS Consul.
func getSegmentMembers(client *consulapi.Client) ([]*consulapi.AgentMember, error) {
	return nil, nil
}
