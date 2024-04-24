// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"fmt"
	"net"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/structs"
	mockpbresource "github.com/hashicorp/consul/grpcmocks/proto-public/pbresource"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

var (
	fakeWrappedErr = fmt.Errorf("fake test error")
)

type testCase struct {
	name             string
	member           serf.Member
	nodeNameOverride string // This is used in the HandleLeftMember test to avoid deregistering ourself

	existingWorkload  *pbresource.Resource
	workloadReadErr   bool
	workloadWriteErr  bool
	workloadDeleteErr bool

	existingHealthStatus *pbresource.Resource
	healthstatusReadErr  bool
	healthstatusWriteErr bool

	mutatedWorkload     *pbresource.Resource // leaving one of these out means the mock expects not to have a write/delete called
	mutatedHealthStatus *pbresource.Resource
	expErr              string
}

func Test_HandleAliveMember(t *testing.T) {
	t.Parallel()

	run := func(t *testing.T, tt testCase) {
		client := mockpbresource.NewResourceServiceClient(t)
		mockClient := client.EXPECT()

		// Build mock expectations based on the order of HandleAliveMember resource calls
		setupReadExpectation(t, mockClient, getTestWorkloadId(), tt.existingWorkload, tt.workloadReadErr)
		setupWriteExpectation(t, mockClient, tt.mutatedWorkload, tt.workloadWriteErr)
		if !tt.workloadReadErr && !tt.workloadWriteErr {
			// We expect to bail before this read if there is an error earlier in the function
			setupReadExpectation(t, mockClient, getTestHealthstatusId(), tt.existingHealthStatus, tt.healthstatusReadErr)
		}
		setupWriteExpectation(t, mockClient, tt.mutatedHealthStatus, tt.healthstatusWriteErr)

		registrator := V2ConsulRegistrator{
			Logger:   hclog.New(&hclog.LoggerOptions{}),
			NodeName: "test-server-1",
			Client:   client,
		}

		// Mock join function
		var joinMockCalled bool
		joinMock := func(_ serf.Member, _ *metadata.Server) error {
			joinMockCalled = true
			return nil
		}

		err := registrator.HandleAliveMember(tt.member, acl.DefaultEnterpriseMeta(), joinMock)
		if tt.expErr != "" {
			require.Contains(t, err.Error(), tt.expErr)
		} else {
			require.NoError(t, err)
		}
		require.True(t, joinMockCalled, "the mock join function was not called")
	}

	tests := []testCase{
		{
			name:                "New alive member",
			member:              getTestSerfMember(serf.StatusAlive),
			mutatedWorkload:     getTestWorkload(t),
			mutatedHealthStatus: getTestHealthStatus(t, true),
		},
		{
			name:                 "No updates needed",
			member:               getTestSerfMember(serf.StatusAlive),
			existingWorkload:     getTestWorkload(t),
			existingHealthStatus: getTestHealthStatus(t, true),
		},
		{
			name:                 "Existing Workload and HS need to be updated",
			member:               getTestSerfMember(serf.StatusAlive),
			existingWorkload:     getTestWorkloadWithPort(t, 8301),
			existingHealthStatus: getTestHealthStatus(t, false),
			mutatedWorkload:      getTestWorkload(t),
			mutatedHealthStatus:  getTestHealthStatus(t, true),
		},
		{
			name:                 "Only the HS needs to be updated",
			member:               getTestSerfMember(serf.StatusAlive),
			existingWorkload:     getTestWorkload(t),
			existingHealthStatus: getTestHealthStatus(t, false),
			mutatedHealthStatus:  getTestHealthStatus(t, true),
		},
		{
			name:            "Error reading Workload",
			member:          getTestSerfMember(serf.StatusAlive),
			workloadReadErr: true,
			expErr:          "error checking for existing Workload",
		},
		{
			name:             "Error writing Workload",
			member:           getTestSerfMember(serf.StatusAlive),
			workloadWriteErr: true,
			mutatedWorkload:  getTestWorkload(t),
			expErr:           "failed to write Workload",
		},
		{
			name:                "Error reading HealthStatus",
			member:              getTestSerfMember(serf.StatusAlive),
			healthstatusReadErr: true,
			mutatedWorkload:     getTestWorkload(t),
			expErr:              "error checking for existing HealthStatus",
		},
		{
			name:                 "Error writing HealthStatus",
			member:               getTestSerfMember(serf.StatusAlive),
			healthstatusWriteErr: true,
			mutatedWorkload:      getTestWorkload(t),
			mutatedHealthStatus:  getTestHealthStatus(t, true),
			expErr:               "failed to write HealthStatus",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run(t, tt)
		})
	}
}

func Test_HandleFailedMember(t *testing.T) {
	t.Parallel()

	run := func(t *testing.T, tt testCase) {
		client := mockpbresource.NewResourceServiceClient(t)
		mockClient := client.EXPECT()

		// Build mock expectations based on the order of HandleFailed resource calls
		setupReadExpectation(t, mockClient, getTestWorkloadId(), tt.existingWorkload, tt.workloadReadErr)
		if !tt.workloadReadErr && tt.existingWorkload != nil {
			// We expect to bail before this read if there is an error earlier in the function or there is no workload
			setupReadExpectation(t, mockClient, getTestHealthstatusId(), tt.existingHealthStatus, tt.healthstatusReadErr)
		}
		setupWriteExpectation(t, mockClient, tt.mutatedHealthStatus, tt.healthstatusWriteErr)

		registrator := V2ConsulRegistrator{
			Logger:   hclog.New(&hclog.LoggerOptions{}),
			NodeName: "test-server-1",
			Client:   client,
		}

		err := registrator.HandleFailedMember(tt.member, acl.DefaultEnterpriseMeta())
		if tt.expErr != "" {
			require.Contains(t, err.Error(), tt.expErr)
		} else {
			require.NoError(t, err)
		}
	}

	tests := []testCase{
		{
			name:                "Update non-existent HealthStatus",
			member:              getTestSerfMember(serf.StatusFailed),
			existingWorkload:    getTestWorkload(t),
			mutatedHealthStatus: getTestHealthStatus(t, false),
		},
		{
			name:   "Underlying Workload does not exist",
			member: getTestSerfMember(serf.StatusFailed),
		},
		{
			name:                 "Update an existing HealthStatus",
			member:               getTestSerfMember(serf.StatusFailed),
			existingWorkload:     getTestWorkload(t),
			existingHealthStatus: getTestHealthStatus(t, true),
			mutatedHealthStatus:  getTestHealthStatus(t, false),
		},
		{
			name:                 "HealthStatus is already critical - no updates needed",
			member:               getTestSerfMember(serf.StatusFailed),
			existingWorkload:     getTestWorkload(t),
			existingHealthStatus: getTestHealthStatus(t, false),
		},
		{
			name:            "Error reading Workload",
			member:          getTestSerfMember(serf.StatusFailed),
			workloadReadErr: true,
			expErr:          "error checking for existing Workload",
		},
		{
			name:                "Error reading HealthStatus",
			member:              getTestSerfMember(serf.StatusFailed),
			existingWorkload:    getTestWorkload(t),
			healthstatusReadErr: true,
			expErr:              "error checking for existing HealthStatus",
		},
		{
			name:                 "Error writing HealthStatus",
			member:               getTestSerfMember(serf.StatusFailed),
			existingWorkload:     getTestWorkload(t),
			healthstatusWriteErr: true,
			mutatedHealthStatus:  getTestHealthStatus(t, false),
			expErr:               "failed to write HealthStatus",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run(t, tt)
		})
	}
}

// Test_HandleLeftMember also tests HandleReapMembers, which are the same core logic with some different logs.
func Test_HandleLeftMember(t *testing.T) {
	t.Parallel()

	run := func(t *testing.T, tt testCase) {
		client := mockpbresource.NewResourceServiceClient(t)
		mockClient := client.EXPECT()

		// Build mock expectations based on the order of HandleLeftMember resource calls
		// We check for the override, which we use to skip self de-registration
		if tt.nodeNameOverride == "" {
			setupReadExpectation(t, mockClient, getTestWorkloadId(), tt.existingWorkload, tt.workloadReadErr)
			if tt.existingWorkload != nil && !tt.workloadReadErr {
				setupDeleteExpectation(t, mockClient, tt.mutatedWorkload, tt.workloadDeleteErr)
			}
		}

		nodeName := "test-server-2" // This is not the same as the serf node so we don't dergister ourself.
		if tt.nodeNameOverride != "" {
			nodeName = tt.nodeNameOverride
		}

		registrator := V2ConsulRegistrator{
			Logger:   hclog.New(&hclog.LoggerOptions{}),
			NodeName: nodeName, // We change this so that we don't deregister ourself
			Client:   client,
		}

		// Mock join function
		var removeMockCalled bool
		removeMock := func(_ serf.Member) error {
			removeMockCalled = true
			return nil
		}

		err := registrator.HandleLeftMember(tt.member, acl.DefaultEnterpriseMeta(), removeMock)
		if tt.expErr != "" {
			require.Contains(t, err.Error(), tt.expErr)
		} else {
			require.NoError(t, err)
		}
		require.True(t, removeMockCalled, "the mock remove function was not called")
	}

	tests := []testCase{
		{
			name:             "Remove member",
			member:           getTestSerfMember(serf.StatusAlive),
			existingWorkload: getTestWorkload(t),
			mutatedWorkload:  getTestWorkload(t),
		},
		{
			name:             "Don't deregister ourself",
			member:           getTestSerfMember(serf.StatusAlive),
			nodeNameOverride: "test-server-1",
		},
		{
			name:   "Don't do anything if the Workload is already gone",
			member: getTestSerfMember(serf.StatusAlive),
		},
		{
			name:             "Remove member regardless of Workload payload",
			member:           getTestSerfMember(serf.StatusAlive),
			existingWorkload: getTestWorkloadWithPort(t, 8301),
			mutatedWorkload:  getTestWorkload(t),
		},
		{
			name:            "Error reading Workload",
			member:          getTestSerfMember(serf.StatusAlive),
			workloadReadErr: true,
			expErr:          "error checking for existing Workload",
		},
		{
			name:              "Error deleting Workload",
			member:            getTestSerfMember(serf.StatusAlive),
			workloadDeleteErr: true,
			existingWorkload:  getTestWorkloadWithPort(t, 8301),
			mutatedWorkload:   getTestWorkload(t),
			expErr:            "failed to delete Workload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run(t, tt)
		})
	}
}

func setupReadExpectation(
	t *testing.T,
	mockClient *mockpbresource.ResourceServiceClient_Expecter,
	expectedId *pbresource.ID,
	existingResource *pbresource.Resource,
	sendErr bool) {

	if sendErr {
		mockClient.Read(mock.Anything, mock.Anything).
			Return(nil, fakeWrappedErr).
			Once().
			Run(func(args mock.Arguments) {
				req := args.Get(1).(*pbresource.ReadRequest)
				require.True(t, proto.Equal(expectedId, req.Id))
			})
	} else if existingResource != nil {
		mockClient.Read(mock.Anything, mock.Anything).
			Return(&pbresource.ReadResponse{
				Resource: existingResource,
			}, nil).
			Once().
			Run(func(args mock.Arguments) {
				req := args.Get(1).(*pbresource.ReadRequest)
				require.True(t, proto.Equal(expectedId, req.Id))
			})
	} else {
		mockClient.Read(mock.Anything, mock.Anything).
			Return(nil, status.Error(codes.NotFound, "not found")).
			Once().
			Run(func(args mock.Arguments) {
				req := args.Get(1).(*pbresource.ReadRequest)
				require.True(t, proto.Equal(expectedId, req.Id))
			})
	}
}

func setupWriteExpectation(
	t *testing.T,
	mockClient *mockpbresource.ResourceServiceClient_Expecter,
	expectedResource *pbresource.Resource,
	sendErr bool) {

	// If there is no expected resource, we take that to mean we don't expect any client writes.
	if expectedResource == nil {
		return
	}

	if sendErr {
		mockClient.Write(mock.Anything, mock.Anything).
			Return(nil, fakeWrappedErr).
			Once().
			Run(func(args mock.Arguments) {
				req := args.Get(1).(*pbresource.WriteRequest)
				require.True(t, proto.Equal(expectedResource, req.Resource))
			})
	} else {
		mockClient.Write(mock.Anything, mock.Anything).
			Return(nil, nil).
			Once().
			Run(func(args mock.Arguments) {
				req := args.Get(1).(*pbresource.WriteRequest)
				require.True(t, proto.Equal(expectedResource, req.Resource))
			})
	}
}

func setupDeleteExpectation(
	t *testing.T,
	mockClient *mockpbresource.ResourceServiceClient_Expecter,
	expectedResource *pbresource.Resource,
	sendErr bool) {

	expectedId := expectedResource.GetId()

	if sendErr {
		mockClient.Delete(mock.Anything, mock.Anything).
			Return(nil, fakeWrappedErr).
			Once().
			Run(func(args mock.Arguments) {
				req := args.Get(1).(*pbresource.DeleteRequest)
				require.True(t, proto.Equal(expectedId, req.Id))
			})
	} else {
		mockClient.Delete(mock.Anything, mock.Anything).
			Return(nil, nil).
			Once().
			Run(func(args mock.Arguments) {
				req := args.Get(1).(*pbresource.DeleteRequest)
				require.True(t, proto.Equal(expectedId, req.Id))
			})
	}
}

func getTestWorkload(t *testing.T) *pbresource.Resource {
	return getTestWorkloadWithPort(t, 8300)
}

func getTestWorkloadWithPort(t *testing.T, port int) *pbresource.Resource {
	workload := &pbcatalog.Workload{
		Addresses: []*pbcatalog.WorkloadAddress{
			{Host: "127.0.0.1", Ports: []string{consulPortNameServer}},
		},
		Ports: map[string]*pbcatalog.WorkloadPort{
			consulPortNameServer: {
				Port:     uint32(port),
				Protocol: pbcatalog.Protocol_PROTOCOL_TCP,
			},
		},
	}
	data, err := anypb.New(workload)
	require.NoError(t, err)

	return &pbresource.Resource{
		Id:   getTestWorkloadId(),
		Data: data,
		Metadata: map[string]string{
			"read_replica":          "false",
			"raft_version":          "3",
			"serf_protocol_current": "2",
			"serf_protocol_min":     "1",
			"serf_protocol_max":     "5",
			"version":               "1.18.0",
			"grpc_port":             "8502",
		},
	}
}

func getTestWorkloadId() *pbresource.ID {
	return &pbresource.ID{
		Tenancy: resource.DefaultNamespacedTenancy(),
		Type:    pbcatalog.WorkloadType,
		Name:    "consul-server-72af047d-1857-2493-969e-53614a70b25a",
	}
}

func getTestHealthStatus(t *testing.T, passing bool) *pbresource.Resource {
	healthStatus := &pbcatalog.HealthStatus{
		Type:        string(structs.SerfCheckID),
		Description: structs.SerfCheckName,
	}

	if passing {
		healthStatus.Status = pbcatalog.Health_HEALTH_PASSING
		healthStatus.Output = structs.SerfCheckAliveOutput
	} else {
		healthStatus.Status = pbcatalog.Health_HEALTH_CRITICAL
		healthStatus.Output = structs.SerfCheckFailedOutput
	}

	data, err := anypb.New(healthStatus)
	require.NoError(t, err)

	return &pbresource.Resource{
		Id:    getTestHealthstatusId(),
		Data:  data,
		Owner: getTestWorkloadId(),
	}
}

func getTestHealthstatusId() *pbresource.ID {
	return &pbresource.ID{
		Tenancy: resource.DefaultNamespacedTenancy(),
		Type:    pbcatalog.HealthStatusType,
		Name:    "consul-server-72af047d-1857-2493-969e-53614a70b25a",
	}
}

func getTestSerfMember(status serf.MemberStatus) serf.Member {
	return serf.Member{
		Name: "test-server-1",
		Addr: net.ParseIP("127.0.0.1"),
		Port: 8300,
		// representative tags from a local dev deployment of ENT
		Tags: map[string]string{
			"vsn_min":       "2",
			"vsn":           "2",
			"acls":          "1",
			"ft_si":         "1",
			"raft_vsn":      "3",
			"grpc_port":     "8502",
			"wan_join_port": "8500",
			"dc":            "dc1",
			"segment":       "",
			"id":            "72af047d-1857-2493-969e-53614a70b25a",
			"ft_admpart":    "1",
			"role":          "consul",
			"build":         "1.18.0",
			"ft_ns":         "1",
			"vsn_max":       "3",
			"bootstrap":     "1",
			"expect":        "1",
			"port":          "8300",
		},
		Status:      status,
		ProtocolMin: 1,
		ProtocolMax: 5,
		ProtocolCur: 2,
		DelegateMin: 2,
		DelegateMax: 5,
		DelegateCur: 4,
	}
}

// Test_ResourceCmpOptions_GeneratedFieldInsensitive makes sure are protocmp options are working as expected.
func Test_ResourceCmpOptions_GeneratedFieldInsensitive(t *testing.T) {
	t.Parallel()

	res1 := getTestWorkload(t)
	res2 := getTestWorkload(t)

	// Modify the generated fields
	res2.Id.Uid = "123456"
	res2.Version = "789"
	res2.Generation = "millenial"
	res2.Status = map[string]*pbresource.Status{
		"foo": {ObservedGeneration: "124"},
	}

	require.True(t, cmp.Equal(res1, res2, resourceCmpOptions...))

	res1.Metadata["foo"] = "bar"

	require.False(t, cmp.Equal(res1, res2, resourceCmpOptions...))
}

// Test gRPC Error Codes Conditions
func Test_grpcNotFoundErr(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name: "Nil Error",
		},
		{
			name: "Nonsense Error",
			err:  fmt.Errorf("boooooo!"),
		},
		{
			name: "gRPC Permission Denied Error",
			err:  status.Error(codes.PermissionDenied, "permission denied is not NotFound"),
		},
		{
			name:     "gRPC NotFound Error",
			err:      status.Error(codes.NotFound, "bingo: not found"),
			expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, grpcNotFoundErr(tt.err))
		})
	}
}
