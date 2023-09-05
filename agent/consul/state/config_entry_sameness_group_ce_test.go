// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

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
