// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1
package authtest

import (
	"testing"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/auth"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource/reaper"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/testutil"
)

func runInMemResourceService(t *testing.T) pbresource.ResourceServiceClient {
	t.Helper()

	ctx := testutil.TestContext(t)

	// Create the in-mem resource service
	client := svctest.RunResourceService(t, auth.RegisterTypes)

	// Setup/Run the controller manager
	mgr := controller.NewManager(client, testutil.Logger(t))

	// We also depend on the reaper to take care of cleaning up owned health statuses and
	// service endpoints so we must enable that controller as well
	reaper.RegisterControllers(mgr)
	mgr.SetRaftLeader(true)
	go mgr.Run(ctx)

	return client
}

func TestResource_Validation(t *testing.T) {
	client := runInMemResourceService(t)
	RunAuthV1Alpha1IntegrationTest(t, client)
}
