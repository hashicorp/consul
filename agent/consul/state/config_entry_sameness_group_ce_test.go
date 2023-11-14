// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package state

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
)

func TestStore_SamenessGroup_checkSamenessGroup(t *testing.T) {
	s := testStateStore(t)
	err := s.EnsureConfigEntry(0, &structs.SamenessGroupConfigEntry{
		Name: "sg1",
	})
	require.ErrorContains(t, err, "sameness-groups are an enterprise-only feature")
}
