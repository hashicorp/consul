// +build !ent

package command

import (
	consulapi "github.com/hashicorp/consul/api"
	"fmt"
)

// getSegmentMembers returns an empty list since network segments are not
// supported in OSS Consul.
func getSegmentMembers(client *consulapi.Client) ([]*consulapi.AgentMember, error) {
	members, err := client.Agent().MembersOpts(consulapi.MembersOpts{})
	if err != nil {
		return nil, fmt.Errorf("Error retrieving members: %s", err)
	}

	return members, nil
}
