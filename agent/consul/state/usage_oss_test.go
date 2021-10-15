// +build !consulent

package state

import (
	"testing"

	"github.com/hashicorp/consul/agent/structs"

	"github.com/stretchr/testify/require"
)

func TestStateStore_Usage_ServiceUsage(t *testing.T) {
	s := testStateStore(t)

	testRegisterNode(t, s, 0, "node1")
	testRegisterNode(t, s, 1, "node2")
	testRegisterService(t, s, 8, "node1", "service1")
	testRegisterService(t, s, 9, "node2", "service1")
	testRegisterService(t, s, 10, "node2", "service2")
	testRegisterSidecarProxy(t, s, 11, "node1", "service1")
	testRegisterSidecarProxy(t, s, 12, "node2", "service1")
	testRegisterConnectNativeService(t, s, 13, "node1", "service-native")
	testRegisterConnectNativeService(t, s, 14, "node2", "service-native")
	testRegisterConnectNativeService(t, s, 15, "node2", "service-native-1")

	idx, usage, err := s.ServiceUsage()
	require.NoError(t, err)
	require.Equal(t, idx, uint64(15))
	require.Equal(t, 5, usage.Services)
	require.Equal(t, 8, usage.ServiceInstances)
	require.Equal(t, 2, usage.ConnectServiceInstances[string(structs.ServiceKindConnectProxy)])
	require.Equal(t, 3, usage.ConnectServiceInstances[connectNativeInstancesTable])
}
