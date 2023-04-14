// Generated.

package integration_test

import (
	"testing"
)

func TestAPI_HealthNode_Noncontainerized(t *testing.T) {
	t.Parallel()
	c, s := NewClientServer(t)
	ts := &cstestAPI_HealthNode{}
	ts.assemble(t, c, s)
	ts.act(t, c, s)
	ts.assert(t, c, s)
}

func TestAPI_HealthNode_Containerized(t *testing.T) {
	t.Parallel()
	c, s := NewClusterTestServerAdapter(t)
	ts := &cstestAPI_HealthNode{}
	ts.assemble(t, c, s)
	ts.act(t, c, s)
	ts.assert(t, c, s)
}

func TestAPI_HealthNode_ContainerizedUpgrade(t *testing.T) {
	t.Parallel()
	c, s := NewClusterTestServerAdapter(t)
	ts := &cstestAPI_HealthNode{}
	ts.assemble(t, c, s)
	ts.act(t, c, s)
	ts.assert(t, c, s)

	s.Upgrade(t)
	ts.assert(t, c, s)
}
