// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package sidecarproxy

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/testutil"
)

func createService(
	t *testing.T,
	client *resourcetest.Client,
	name string,
	tenancy *pbresource.Tenancy,
	exactSelector string,
	ports []*pbcatalog.ServicePort,
	vips []string,
	workloadIdentities []string,
	deferStatusUpdate bool,
) (*pbresource.Resource, *pbcatalog.Service, func() *pbresource.Resource) {
	data := &pbcatalog.Service{
		Workloads: &pbcatalog.WorkloadSelector{
			Names: []string{exactSelector},
		},
		Ports:      ports,
		VirtualIps: vips,
	}

	var status *pbresource.Status
	if deferStatusUpdate {
		status = &pbresource.Status{
			Conditions: []*pbresource.Condition{{
				Type:    catalog.StatusConditionBoundIdentities,
				State:   pbresource.Condition_STATE_TRUE,
				Message: "",
			}},
		}
	} else {
		status = &pbresource.Status{
			Conditions: []*pbresource.Condition{{
				Type:    catalog.StatusConditionBoundIdentities,
				State:   pbresource.Condition_STATE_TRUE,
				Message: strings.Join(workloadIdentities, ","),
			}},
		}
	}

	res := resourcetest.Resource(pbcatalog.ServiceType, name).
		WithTenancy(tenancy).
		WithData(t, data).
		WithStatus(catalog.EndpointsStatusKey, status).
		Write(t, client)

	var statusUpdate = func() *pbresource.Resource { return res }
	if deferStatusUpdate {
		statusUpdate = func() *pbresource.Resource {
			ctx := testutil.TestContext(t)

			status := &pbresource.Status{
				ObservedGeneration: res.Generation,
				Conditions: []*pbresource.Condition{{
					Type:    catalog.StatusConditionBoundIdentities,
					State:   pbresource.Condition_STATE_TRUE,
					Message: strings.Join(workloadIdentities, ","),
				}},
			}
			resp, err := client.WriteStatus(ctx, &pbresource.WriteStatusRequest{
				Id:     res.Id,
				Key:    catalog.EndpointsStatusKey,
				Status: status,
			})
			require.NoError(t, err)
			return resp.Resource
		}
	}

	return res, data, statusUpdate
}
