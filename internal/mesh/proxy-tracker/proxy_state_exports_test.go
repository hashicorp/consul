// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxytracker

import (
	"github.com/hashicorp/consul/acl"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestProxyState_Authorize(t *testing.T) {
	testIdentity := &pbresource.Reference{
		Type: &pbresource.Type{
			Group:        "mesh",
			GroupVersion: "v1alpha1",
			Kind:         "Identity",
		},
		Tenancy: &pbresource.Tenancy{
			Partition: "default",
			Namespace: "default",
		},
		Name: "test-identity",
	}

	type testCase struct {
		description          string
		proxyState           *ProxyState
		configureAuthorizer  func(authorizer *acl.MockAuthorizer)
		expectedErrorMessage string
	}
	testsCases := []testCase{
		{
			description: "ProxyState - if identity write is allowed for the workload then allow.",
			proxyState: &ProxyState{
				ProxyState: &pbmesh.ProxyState{
					Identity: testIdentity,
				},
			},
			expectedErrorMessage: "",
			configureAuthorizer: func(authz *acl.MockAuthorizer) {
				authz.On("IdentityWrite", testIdentity.Name, mock.Anything).Return(acl.Allow)
			},
		},
		{
			description: "ProxyState - if identity write is not allowed for the workload then deny.",
			proxyState: &ProxyState{
				ProxyState: &pbmesh.ProxyState{
					Identity: testIdentity,
				},
			},
			expectedErrorMessage: "Permission denied: token with AccessorID '' lacks permission 'identity:write' on \"test-identity\"",
			configureAuthorizer: func(authz *acl.MockAuthorizer) {
				authz.On("IdentityWrite", testIdentity.Name, mock.Anything).Return(acl.Deny)
			},
		},
	}
	for _, tc := range testsCases {
		t.Run(tc.description, func(t *testing.T) {
			authz := &acl.MockAuthorizer{}
			authz.On("ToAllow").Return(acl.AllowAuthorizer{Authorizer: authz})
			tc.configureAuthorizer(authz)
			err := tc.proxyState.Authorize(authz)
			errMsg := ""
			if err != nil {
				errMsg = err.Error()
			}
			// using contains because Enterprise tests append the parition and namespace
			// information to the message.
			require.True(t, strings.Contains(errMsg, tc.expectedErrorMessage))
		})
	}
}
