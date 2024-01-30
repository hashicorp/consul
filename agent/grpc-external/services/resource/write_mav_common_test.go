// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/anypb"

	svc "github.com/hashicorp/consul/agent/grpc-external/services/resource"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbdemov2 "github.com/hashicorp/consul/proto/private/pbdemo/v2"
)

// Common test structs and test cases shared by the Write and MutateAndValidate RPCs
// only. These are not intended to be used by other tests.

type resourceValidTestCase struct {
	modFn       func(artist, recordLabel *pbresource.Resource) *pbresource.Resource
	errContains string
}

func resourceValidTestCases(t *testing.T) map[string]resourceValidTestCase {
	return map[string]resourceValidTestCase{
		"no resource": {
			modFn: func(_, _ *pbresource.Resource) *pbresource.Resource {
				return nil
			},
			errContains: "resource is required",
		},
		"no id": {
			modFn: func(artist, _ *pbresource.Resource) *pbresource.Resource {
				artist.Id = nil
				return artist
			},
			errContains: "resource.id is required",
		},
		"no type": {
			modFn: func(artist, _ *pbresource.Resource) *pbresource.Resource {
				artist.Id.Type = nil
				return artist
			},
			errContains: "resource.id.type is required",
		},
		"no name": {
			modFn: func(artist, _ *pbresource.Resource) *pbresource.Resource {
				artist.Id.Name = ""
				return artist
			},
			errContains: "resource.id.name invalid",
		},
		"name is mixed case": {
			modFn: func(artist, _ *pbresource.Resource) *pbresource.Resource {
				artist.Id.Name = "MixedCaseNotAllowed"
				return artist
			},
			errContains: "resource.id.name invalid",
		},
		"name too long": {
			modFn: func(artist, _ *pbresource.Resource) *pbresource.Resource {
				artist.Id.Name = strings.Repeat("a", resource.MaxNameLength+1)
				return artist
			},
			errContains: "resource.id.name invalid",
		},
		"wrong data type": {
			modFn: func(artist, _ *pbresource.Resource) *pbresource.Resource {
				var err error
				artist.Data, err = anypb.New(&pbdemov2.Album{})
				require.NoError(t, err)
				return artist
			},
			errContains: "resource.data is of wrong type",
		},
		"partition is mixed case": {
			modFn: func(artist, _ *pbresource.Resource) *pbresource.Resource {
				artist.Id.Tenancy.Partition = "Default"
				return artist
			},
			errContains: "resource.id.tenancy.partition invalid",
		},
		"partition too long": {
			modFn: func(artist, _ *pbresource.Resource) *pbresource.Resource {
				artist.Id.Tenancy.Partition = strings.Repeat("p", resource.MaxNameLength+1)
				return artist
			},
			errContains: "resource.id.tenancy.partition invalid",
		},
		"namespace is mixed case": {
			modFn: func(artist, _ *pbresource.Resource) *pbresource.Resource {
				artist.Id.Tenancy.Namespace = "Default"
				return artist
			},
			errContains: "resource.id.tenancy.namespace invalid",
		},
		"namespace too long": {
			modFn: func(artist, _ *pbresource.Resource) *pbresource.Resource {
				artist.Id.Tenancy.Namespace = strings.Repeat("n", resource.MaxNameLength+1)
				return artist
			},
			errContains: "resource.id.tenancy.namespace invalid",
		},
		"fail validation hook": {
			modFn: func(artist, _ *pbresource.Resource) *pbresource.Resource {
				buffer := &pbdemov2.Artist{}
				require.NoError(t, artist.Data.UnmarshalTo(buffer))
				buffer.Name = "" // name cannot be empty
				require.NoError(t, artist.Data.MarshalFrom(buffer))
				return artist
			},
			errContains: "artist.name required",
		},
		"partition scope with non-empty namespace": {
			modFn: func(_, recordLabel *pbresource.Resource) *pbresource.Resource {
				recordLabel.Id.Tenancy.Namespace = "bogus"
				return recordLabel
			},
			errContains: "cannot have a namespace",
		},
	}
}

type ownerValidTestCase struct {
	modFn         func(res *pbresource.Resource)
	errorContains string
}

func ownerValidationTestCases(t *testing.T) map[string]ownerValidTestCase {
	return map[string]ownerValidTestCase{
		"no owner type": {
			modFn:         func(res *pbresource.Resource) { res.Owner.Type = nil },
			errorContains: "resource.owner.type is required",
		},
		"no owner name": {
			modFn:         func(res *pbresource.Resource) { res.Owner.Name = "" },
			errorContains: "resource.owner.name invalid",
		},
		"mixed case owner name": {
			modFn:         func(res *pbresource.Resource) { res.Owner.Name = strings.ToUpper(res.Owner.Name) },
			errorContains: "resource.owner.name invalid",
		},
		"owner name too long": {
			modFn: func(res *pbresource.Resource) {
				res.Owner.Name = strings.Repeat("a", resource.MaxNameLength+1)
			},
			errorContains: "resource.owner.name invalid",
		},
		"owner partition is mixed case": {
			modFn: func(res *pbresource.Resource) {
				res.Owner.Tenancy.Partition = "Default"
			},
			errorContains: "resource.owner.tenancy.partition invalid",
		},
		"owner partition too long": {
			modFn: func(res *pbresource.Resource) {
				res.Owner.Tenancy.Partition = strings.Repeat("p", resource.MaxNameLength+1)
			},
			errorContains: "resource.owner.tenancy.partition invalid",
		},
		"owner namespace is mixed case": {
			modFn: func(res *pbresource.Resource) {
				res.Owner.Tenancy.Namespace = "Default"
			},
			errorContains: "resource.owner.tenancy.namespace invalid",
		},
		"owner namespace too long": {
			modFn: func(res *pbresource.Resource) {
				res.Owner.Tenancy.Namespace = strings.Repeat("n", resource.MaxNameLength+1)
			},
			errorContains: "resource.owner.tenancy.namespace invalid",
		},
	}
}

// Test case struct shared by MutateAndValidate and Write success test cases
type mavOrWriteSuccessTestCase struct {
	modFn           func(artist, recordLabel *pbresource.Resource) *pbresource.Resource
	expectedTenancy *pbresource.Tenancy
}

// Test case struct shared by MutateAndValidate and Write success test cases
func mavOrWriteSuccessTestCases(t *testing.T) map[string]mavOrWriteSuccessTestCase {
	return map[string]mavOrWriteSuccessTestCase{
		"namespaced resource provides nonempty partition and namespace": {
			modFn: func(artist, _ *pbresource.Resource) *pbresource.Resource {
				return artist
			},
			expectedTenancy: resource.DefaultNamespacedTenancy(),
		},
		"namespaced resource inherits tokens partition when empty": {
			modFn: func(artist, _ *pbresource.Resource) *pbresource.Resource {
				artist.Id.Tenancy.Partition = ""
				return artist
			},
			expectedTenancy: resource.DefaultNamespacedTenancy(),
		},
		"namespaced resource inherits tokens namespace when empty": {
			modFn: func(artist, _ *pbresource.Resource) *pbresource.Resource {
				artist.Id.Tenancy.Namespace = ""
				return artist
			},
			expectedTenancy: resource.DefaultNamespacedTenancy(),
		},
		"namespaced resource inherits tokens partition and namespace when empty": {
			modFn: func(artist, _ *pbresource.Resource) *pbresource.Resource {
				artist.Id.Tenancy.Partition = ""
				artist.Id.Tenancy.Namespace = ""
				return artist
			},
			expectedTenancy: resource.DefaultNamespacedTenancy(),
		},
		"namespaced resource inherits tokens partition and namespace when tenancy nil": {
			modFn: func(artist, _ *pbresource.Resource) *pbresource.Resource {
				artist.Id.Tenancy = nil
				return artist
			},
			expectedTenancy: resource.DefaultNamespacedTenancy(),
		},
		"partitioned resource provides nonempty partition": {
			modFn: func(_, recordLabel *pbresource.Resource) *pbresource.Resource {
				return recordLabel
			},
			expectedTenancy: resource.DefaultPartitionedTenancy(),
		},
		"partitioned resource inherits tokens partition when empty": {
			modFn: func(_, recordLabel *pbresource.Resource) *pbresource.Resource {
				recordLabel.Id.Tenancy.Partition = ""
				return recordLabel
			},
			expectedTenancy: resource.DefaultPartitionedTenancy(),
		},
		"partitioned resource inherits tokens partition when tenancy nil": {
			modFn: func(_, recordLabel *pbresource.Resource) *pbresource.Resource {
				recordLabel.Id.Tenancy = nil
				return recordLabel
			},
			expectedTenancy: resource.DefaultPartitionedTenancy(),
		},
	}
}

// Test case struct shared by MutateAndValidate and Write test cases where tenancy is not found
type mavOrWriteTenancyNotFoundTestCase map[string]struct {
	modFn       func(artist, recordLabel *pbresource.Resource) *pbresource.Resource
	errCode     codes.Code
	errContains string
}

// Test case struct shared by MutateAndValidate and Write test cases where tenancy is not found
func mavOrWriteTenancyNotFoundTestCases(t *testing.T) mavOrWriteTenancyNotFoundTestCase {
	return mavOrWriteTenancyNotFoundTestCase{
		"namespaced resource provides nonexistant partition": {
			modFn: func(artist, _ *pbresource.Resource) *pbresource.Resource {
				artist.Id.Tenancy.Partition = "boguspartition"
				return artist
			},
			errCode:     codes.InvalidArgument,
			errContains: "partition not found",
		},
		"namespaced resource provides nonexistant namespace": {
			modFn: func(artist, _ *pbresource.Resource) *pbresource.Resource {
				artist.Id.Tenancy.Namespace = "bogusnamespace"
				return artist
			},
			errCode:     codes.InvalidArgument,
			errContains: "namespace not found",
		},
		"partitioned resource provides nonexistant partition": {
			modFn: func(_, recordLabel *pbresource.Resource) *pbresource.Resource {
				recordLabel.Id.Tenancy.Partition = "boguspartition"
				return recordLabel
			},
			errCode:     codes.InvalidArgument,
			errContains: "partition not found",
		},
	}
}

type mavOrWriteTenancyMarkedForDeletionTestCase struct {
	modFn       func(artist, recordLabel *pbresource.Resource, mockTenancyBridge *svc.MockTenancyBridge) *pbresource.Resource
	errContains string
}

func mavOrWriteTenancyMarkedForDeletionTestCases(t *testing.T) map[string]mavOrWriteTenancyMarkedForDeletionTestCase {
	return map[string]mavOrWriteTenancyMarkedForDeletionTestCase{
		"namespaced resources partition marked for deletion": {
			modFn: func(artist, _ *pbresource.Resource, mockTenancyBridge *svc.MockTenancyBridge) *pbresource.Resource {
				mockTenancyBridge.On("IsPartitionMarkedForDeletion", "ap1").Return(true, nil)
				return artist
			},
			errContains: "tenancy marked for deletion",
		},
		"namespaced resources namespace marked for deletion": {
			modFn: func(artist, _ *pbresource.Resource, mockTenancyBridge *svc.MockTenancyBridge) *pbresource.Resource {
				mockTenancyBridge.On("IsPartitionMarkedForDeletion", "ap1").Return(false, nil)
				mockTenancyBridge.On("IsNamespaceMarkedForDeletion", "ap1", "ns1").Return(true, nil)
				return artist
			},
			errContains: "tenancy marked for deletion",
		},
		"partitioned resources partition marked for deletion": {
			modFn: func(_, recordLabel *pbresource.Resource, mockTenancyBridge *svc.MockTenancyBridge) *pbresource.Resource {
				mockTenancyBridge.On("IsPartitionMarkedForDeletion", "ap1").Return(true, nil)
				return recordLabel
			},
			errContains: "tenancy marked for deletion",
		},
	}
}
