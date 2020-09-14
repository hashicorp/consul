// +build !consulent

package state

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStateStore_Usage_ServiceUsage(t *testing.T) {
	s := testStateStore(t)

	testRegisterNode(t, s, 0, "node1")
	testRegisterNode(t, s, 1, "node2")
	testRegisterService(t, s, 8, "node1", "service1")
	testRegisterService(t, s, 9, "node2", "service1")
	testRegisterService(t, s, 10, "node2", "service2")

	idx, usage, err := s.ServiceUsage()
	require.NoError(t, err)
	require.Equal(t, idx, uint64(10))
	require.Equal(t, 2, usage.Services)
	require.Equal(t, 3, usage.ServiceInstances)
}
