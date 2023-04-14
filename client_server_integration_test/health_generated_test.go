// Generated.

package integration_test

import (
	"testing"
)

func TestAPI_HealthNode_Noncontainerized(t *testing.T) {
	c, s := NewClientServer(t)
	cstestAPI_HealthNode(t, c, s)
}

func TestAPI_HealthNode_Containerized(t *testing.T) {
	c, s := NewClusterTestServerAdapter(t)
	cstestAPI_HealthNode(t, c, s)
}

/* TODO
func TestAPI_HealthNode_ContainerizedUpgrade(t *testing.T) {
	c, s := topology.NewTestServerAdapter(t)
	cstestAPI_HealthNode(t, c, s)

	s.StandardUpgrade()
}
*/
