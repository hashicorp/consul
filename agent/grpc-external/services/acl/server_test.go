// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package acl

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/hashicorp/go-uuid"

	"github.com/hashicorp/consul/acl"
	structs "github.com/hashicorp/consul/agent/structs"
)

func generateID(t *testing.T) string {
	t.Helper()

	id, err := uuid.GenerateUUID()
	require.NoError(t, err)

	return id
}

func noopForwardRPC(structs.RPCInfo, func(*grpc.ClientConn) error) (bool, error) {
	return false, nil
}

func noopValidateEnterpriseRequest(*acl.EnterpriseMeta, bool) error {
	return nil
}

func noopLocalTokensEnabled() bool {
	return true
}

func noopACLsEnabled() bool {
	return true
}
