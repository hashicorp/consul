// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"fmt"
	"testing"

	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
)

func createDNSPolicyResource(t *testing.T, data protoreflect.ProtoMessage) *pbresource.Resource {
	res := &pbresource.Resource{
		Id: &pbresource.ID{
			Type: DNSPolicyType,
			Tenancy: &pbresource.Tenancy{
				Partition: "default",
				Namespace: "default",
				PeerName:  "local",
			},
			Name: "test-policy",
		},
	}

	var err error
	res.Data, err = anypb.New(data)
	require.NoError(t, err)
	return res
}

func TestValidateDNSPolicy_Ok(t *testing.T) {
	data := &pbcatalog.DNSPolicy{
		Workloads: &pbcatalog.WorkloadSelector{
			Prefixes: []string{""},
		},
		Weights: &pbcatalog.Weights{
			Passing: 3,
			Warning: 0,
		},
	}

	res := createDNSPolicyResource(t, data)

	err := ValidateDNSPolicy(res)
	require.NoError(t, err)
}

func TestValidateDNSPolicy_ParseError(t *testing.T) {
	// Any type other than the DNSPolicy type would work
	// to cause the error we are expecting
	data := &pbcatalog.IP{Address: "198.18.0.1"}

	res := createDNSPolicyResource(t, data)

	err := ValidateDNSPolicy(res)
	require.Error(t, err)
	require.ErrorAs(t, err, &resource.ErrDataParse{})
}

func TestValidateDNSPolicy_MissingWeights(t *testing.T) {
	data := &pbcatalog.DNSPolicy{
		Workloads: &pbcatalog.WorkloadSelector{
			Prefixes: []string{""},
		},
	}

	res := createDNSPolicyResource(t, data)

	err := ValidateDNSPolicy(res)
	require.Error(t, err)
	expected := resource.ErrInvalidField{
		Name:    "weights",
		Wrapped: resource.ErrMissing,
	}
	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}

func TestValidateDNSPolicy_InvalidPassingWeight(t *testing.T) {
	for _, weight := range []uint32{0, 1000000} {
		t.Run(fmt.Sprintf("%d", weight), func(t *testing.T) {
			data := &pbcatalog.DNSPolicy{
				Workloads: &pbcatalog.WorkloadSelector{
					Prefixes: []string{""},
				},
				Weights: &pbcatalog.Weights{
					Passing: weight,
				},
			}

			res := createDNSPolicyResource(t, data)

			err := ValidateDNSPolicy(res)
			require.Error(t, err)
			expected := resource.ErrInvalidField{
				Name:    "passing",
				Wrapped: errDNSPassingWeightOutOfRange,
			}
			var actual resource.ErrInvalidField
			require.ErrorAs(t, err, &actual)
			require.Equal(t, "weights", actual.Name)
			err = actual.Unwrap()
			require.ErrorAs(t, err, &actual)
			require.Equal(t, expected, actual)
		})
	}
}

func TestValidateDNSPolicy_InvalidWarningWeight(t *testing.T) {
	data := &pbcatalog.DNSPolicy{
		Workloads: &pbcatalog.WorkloadSelector{
			Prefixes: []string{""},
		},
		Weights: &pbcatalog.Weights{
			Passing: 1,
			Warning: 1000000,
		},
	}

	res := createDNSPolicyResource(t, data)

	err := ValidateDNSPolicy(res)
	require.Error(t, err)
	expected := resource.ErrInvalidField{
		Name:    "warning",
		Wrapped: errDNSWarningWeightOutOfRange,
	}
	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.Equal(t, "weights", actual.Name)
	err = actual.Unwrap()
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}

func TestValidateDNSPolicy_EmptySelector(t *testing.T) {
	data := &pbcatalog.DNSPolicy{
		Weights: &pbcatalog.Weights{
			Passing: 10,
			Warning: 3,
		},
	}

	res := createDNSPolicyResource(t, data)

	err := ValidateDNSPolicy(res)
	require.Error(t, err)
	expected := resource.ErrInvalidField{
		Name:    "workloads",
		Wrapped: resource.ErrEmpty,
	}
	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}
