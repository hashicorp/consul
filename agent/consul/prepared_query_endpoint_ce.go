// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package consul

import (
	"fmt"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

func parseSameness(svc *structs.ServiceQuery) error {
	if svc.SamenessGroup != "" {
		return fmt.Errorf("sameness-groups are an enterprise-only feature")
	}
	return nil
}

func (sl stateLookup) samenessGroupLookup(_ string, _ acl.EnterpriseMeta) (uint64, *structs.SamenessGroupConfigEntry, error) {
	return 0, nil, nil
}

// GetSamenessGroupFailoverTargets supports Sameness Groups an enterprise only feature. This satisfies the queryServer interface
func (q *queryServerWrapper) GetSamenessGroupFailoverTargets(_ string, _ acl.EnterpriseMeta) ([]structs.QueryFailoverTarget, error) {
	return []structs.QueryFailoverTarget{}, nil
}

func querySameness(_ queryServer,
	_ structs.PreparedQuery,
	_ *structs.PreparedQueryExecuteRequest,
	_ *structs.PreparedQueryExecuteResponse) error {

	return nil
}
