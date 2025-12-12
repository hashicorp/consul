// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package state

import (
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestStore_SamenessGroup_checkSamenessGroup(t *testing.T) {
	s := testStateStore(t)
	err := s.EnsureConfigEntry(0, &structs.SamenessGroupConfigEntry{
		Name: "sg1",
	})
	require.ErrorContains(t, err, "sameness-groups are an enterprise-only feature")
}
