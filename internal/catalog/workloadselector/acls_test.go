// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package workloadselector

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func TestACLHooks(t *testing.T) {
	suite.Run(t, new(aclHookSuite))
}

type aclHookSuite struct {
	suite.Suite

	hooks *resource.ACLHooks
	authz *acl.MockAuthorizer
	ctx   *acl.AuthorizerContext
	res   *pbresource.Resource
}

func (suite *aclHookSuite) SetupTest() {
	suite.authz = new(acl.MockAuthorizer)

	suite.authz.On("ToAllowAuthorizer").Return(acl.AllowAuthorizer{Authorizer: suite.authz, AccessorID: "862270e5-7d7b-4583-98bc-4d14810cc158"})

	suite.ctx = &acl.AuthorizerContext{}
	acl.DefaultEnterpriseMeta().FillAuthzContext(suite.ctx)

	suite.hooks = ACLHooks[*pbcatalog.Service]()

	suite.res = resourcetest.Resource(pbcatalog.ServiceType, "foo").
		WithData(suite.T(), &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{"api-"},
				Names:    []string{"bar"},
			},
		}).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()
}

func (suite *aclHookSuite) TeardownTest() {
	suite.authz.AssertExpectations(suite.T())
}

func (suite *aclHookSuite) TestReadHook_Allowed() {
	suite.authz.On("ServiceRead", "foo", suite.ctx).
		Return(acl.Allow).
		Once()

	require.NoError(suite.T(), suite.hooks.Read(suite.authz, suite.ctx, suite.res.Id, nil))
}

func (suite *aclHookSuite) TestReadHook_Denied() {
	suite.authz.On("ServiceRead", "foo", suite.ctx).
		Return(acl.Deny).
		Once()

	require.Error(suite.T(), suite.hooks.Read(suite.authz, suite.ctx, suite.res.Id, nil))
}

func (suite *aclHookSuite) TestWriteHook_ServiceWriteDenied() {
	suite.authz.On("ServiceWrite", "foo", suite.ctx).
		Return(acl.Deny).
		Once()

	require.Error(suite.T(), suite.hooks.Write(suite.authz, suite.ctx, suite.res))
}

func (suite *aclHookSuite) TestWriteHook_ServiceReadNameDenied() {
	suite.authz.On("ServiceWrite", "foo", suite.ctx).
		Return(acl.Allow).
		Once()

	suite.authz.On("ServiceRead", "bar", suite.ctx).
		Return(acl.Deny).
		Once()

	require.Error(suite.T(), suite.hooks.Write(suite.authz, suite.ctx, suite.res))
}

func (suite *aclHookSuite) TestWriteHook_ServiceReadPrefixDenied() {
	suite.authz.On("ServiceWrite", "foo", suite.ctx).
		Return(acl.Allow).
		Once()

	suite.authz.On("ServiceRead", "bar", suite.ctx).
		Return(acl.Allow).
		Once()

	suite.authz.On("ServiceReadPrefix", "api-", suite.ctx).
		Return(acl.Deny).
		Once()

	require.Error(suite.T(), suite.hooks.Write(suite.authz, suite.ctx, suite.res))
}

func (suite *aclHookSuite) TestWriteHook_Allowed() {
	suite.authz.On("ServiceWrite", "foo", suite.ctx).
		Return(acl.Allow).
		Once()

	suite.authz.On("ServiceRead", "bar", suite.ctx).
		Return(acl.Allow).
		Once()

	suite.authz.On("ServiceReadPrefix", "api-", suite.ctx).
		Return(acl.Allow).
		Once()

	require.NoError(suite.T(), suite.hooks.Write(suite.authz, suite.ctx, suite.res))
}
