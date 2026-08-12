// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"fmt"

	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/structs"
)

const (
	tableFeatureGatePolicy = "feature-gate-policy"
	tableFeatureGateStatus = "feature-gate-status"
)

func featureGatePolicyTableSchema() *memdb.TableSchema {
	return singletonFeatureGateTableSchema(tableFeatureGatePolicy)
}

func featureGateStatusTableSchema() *memdb.TableSchema {
	return singletonFeatureGateTableSchema(tableFeatureGateStatus)
}

func singletonFeatureGateTableSchema(name string) *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: name,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: true,
				Unique:       true,
				Indexer: &memdb.ConditionalIndex{
					Conditional: func(interface{}) (bool, error) { return true, nil },
				},
			},
		},
	}
}

// FeatureGatePolicyAndStatus returns both singletons from the same read
// transaction and adds both tables to ws when it is non-nil.
func (s *Store) FeatureGatePolicyAndStatus(ws memdb.WatchSet) (uint64, *structs.FeatureGatePolicy, *structs.FeatureGateStatus, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	policyCh, rawPolicy, err := tx.FirstWatch(tableFeatureGatePolicy, indexID)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("failed feature-gate policy lookup: %w", err)
	}
	statusCh, rawStatus, err := tx.FirstWatch(tableFeatureGateStatus, indexID)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("failed feature-gate status lookup: %w", err)
	}
	if ws != nil {
		ws.Add(policyCh)
		ws.Add(statusCh)
	}

	var policy *structs.FeatureGatePolicy
	if rawPolicy != nil {
		policy = rawPolicy.(*structs.FeatureGatePolicy).Clone()
	}
	var status *structs.FeatureGateStatus
	if rawStatus != nil {
		status = rawStatus.(*structs.FeatureGateStatus).Clone()
	}

	var index uint64
	if policy != nil && policy.ModifyIndex > index {
		index = policy.ModifyIndex
	}
	if status != nil && status.ModifyIndex > index {
		index = status.ModifyIndex
	}
	return index, policy, status, nil
}

// FeatureGateUpdate atomically applies policy/status with CAS fencing. It
// returns false, nil when either expected index no longer matches.
func (s *Store) FeatureGateUpdate(idx uint64, req *structs.FeatureGateUpdateRequest) (bool, error) {
	if req == nil || req.Status == nil {
		return false, fmt.Errorf("feature-gate update requires status")
	}

	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	rawPolicy, err := tx.First(tableFeatureGatePolicy, indexID)
	if err != nil {
		return false, fmt.Errorf("failed feature-gate policy lookup: %w", err)
	}
	rawStatus, err := tx.First(tableFeatureGateStatus, indexID)
	if err != nil {
		return false, fmt.Errorf("failed feature-gate status lookup: %w", err)
	}

	var existingPolicy *structs.FeatureGatePolicy
	if rawPolicy != nil {
		existingPolicy = rawPolicy.(*structs.FeatureGatePolicy)
	}
	var existingStatus *structs.FeatureGateStatus
	if rawStatus != nil {
		existingStatus = rawStatus.(*structs.FeatureGateStatus)
	}
	if raftModifyIndex(existingPolicy) != req.ExpectedPolicyIndex || raftModifyIndex(existingStatus) != req.ExpectedStatusIndex {
		return false, nil
	}

	policyIndex := req.ExpectedPolicyIndex
	if req.Policy != nil {
		policy := req.Policy.Clone()
		if existingPolicy == nil {
			policy.CreateIndex = idx
		} else {
			policy.CreateIndex = existingPolicy.CreateIndex
		}
		policy.ModifyIndex = idx
		if err := tx.Insert(tableFeatureGatePolicy, policy); err != nil {
			return false, fmt.Errorf("failed updating feature-gate policy: %w", err)
		}
		policyIndex = idx
	} else if existingPolicy == nil {
		return false, fmt.Errorf("feature-gate status cannot exist without policy")
	}

	status := req.Status.Clone()
	status.PolicyIndex = policyIndex
	if existingStatus == nil {
		status.CreateIndex = idx
	} else {
		status.CreateIndex = existingStatus.CreateIndex
	}
	status.ModifyIndex = idx
	if err := tx.Insert(tableFeatureGateStatus, status); err != nil {
		return false, fmt.Errorf("failed updating feature-gate status: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func raftModifyIndex(value interface{}) uint64 {
	switch typed := value.(type) {
	case *structs.FeatureGatePolicy:
		if typed == nil {
			return 0
		}
		return typed.ModifyIndex
	case *structs.FeatureGateStatus:
		if typed == nil {
			return 0
		}
		return typed.ModifyIndex
	default:
		return 0
	}
}

func (s *Snapshot) FeatureGates() (*structs.FeatureGateSnapshot, error) {
	policy, err := s.tx.First(tableFeatureGatePolicy, indexID)
	if err != nil {
		return nil, err
	}
	status, err := s.tx.First(tableFeatureGateStatus, indexID)
	if err != nil {
		return nil, err
	}
	result := &structs.FeatureGateSnapshot{}
	if policy != nil {
		result.Policy = policy.(*structs.FeatureGatePolicy).Clone()
	}
	if status != nil {
		result.Status = status.(*structs.FeatureGateStatus).Clone()
	}
	if result.Policy == nil && result.Status == nil {
		return nil, nil
	}
	return result, nil
}

func (s *Restore) FeatureGates(snapshot *structs.FeatureGateSnapshot) error {
	if snapshot == nil {
		return nil
	}
	if snapshot.Policy != nil {
		if err := s.tx.Insert(tableFeatureGatePolicy, snapshot.Policy.Clone()); err != nil {
			return fmt.Errorf("failed restoring feature-gate policy: %w", err)
		}
	}
	if snapshot.Status != nil {
		if err := s.tx.Insert(tableFeatureGateStatus, snapshot.Status.Clone()); err != nil {
			return fmt.Errorf("failed restoring feature-gate status: %w", err)
		}
	}
	return nil
}
