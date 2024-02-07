// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package catalogtest

import (
	"testing"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller/controllertest"
	"github.com/hashicorp/consul/internal/multicluster"
	"github.com/hashicorp/consul/internal/resource/reaper"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

var (
	clientOpts = rtest.ConfigureTestCLIFlags()
)

func runInMemResourceServiceAndControllers(t *testing.T) pbresource.ResourceServiceClient {
	t.Helper()

	return controllertest.NewControllerTestBuilder().
		WithResourceRegisterFns(catalog.RegisterTypes, multicluster.RegisterTypes).
		WithControllerRegisterFns(
			reaper.RegisterControllers,
			catalog.RegisterControllers,
		).Run(t)
}

func TestControllers_Integration(t *testing.T) {
	client := runInMemResourceServiceAndControllers(t)
	RunCatalogV2Beta1IntegrationTest(t, client, clientOpts.ClientOptions(t)...)
}

func TestControllers_Lifecycle(t *testing.T) {
	client := runInMemResourceServiceAndControllers(t)
	RunCatalogV2Beta1LifecycleIntegrationTest(t, client, clientOpts.ClientOptions(t)...)
}
