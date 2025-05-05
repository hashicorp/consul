// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"fmt"
	"testing"

	"github.com/hashicorp/go-uuid"
	"github.com/stretchr/testify/require"
)

func GenerateTestResourceID(t *testing.T) string {
	orgID, err := uuid.GenerateUUID()
	require.NoError(t, err)

	projectID, err := uuid.GenerateUUID()
	require.NoError(t, err)

	template := "organization/%s/project/%s/hashicorp.consul.global-network-manager.cluster/test-cluster"
	return fmt.Sprintf(template, orgID, projectID)
}
