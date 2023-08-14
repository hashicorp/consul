// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package routes

import (
	"context"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type PendingResourceStatusUpdate struct {
	ID         *pbresource.ID
	Generation string
	CurrStatus *pbresource.Status

	NewConditions []*pbresource.Condition
}

type PendingStatuses map[resource.ReferenceKey]*PendingResourceStatusUpdate

func (p PendingStatuses) AddConditions(
	rk resource.ReferenceKey,
	res *pbresource.Resource,
	newConditions []*pbresource.Condition,
) {
	state, ok := p[rk]
	if !ok {
		state = &PendingResourceStatusUpdate{
			ID:         res.Id,
			Generation: res.Generation,
			CurrStatus: res.Status[StatusKey],
		}
		p[rk] = state
	}

	state.NewConditions = append(state.NewConditions, newConditions...)
}

func UpdatePendingStatuses(
	ctx context.Context,
	rt controller.Runtime,
	pending PendingStatuses,
) error {
	for _, state := range pending {
		logger := rt.Logger.With("resource", resource.IDToString(state.ID))

		var newStatus *pbresource.Status
		if len(state.NewConditions) > 0 {
			newStatus = &pbresource.Status{
				ObservedGeneration: state.Generation,
				Conditions:         state.NewConditions,
			}
		} else {
			newStatus = &pbresource.Status{
				ObservedGeneration: state.Generation,
				Conditions: []*pbresource.Condition{
					ConditionXRouteOK,
				},
			}
		}
		if resource.EqualStatus(state.CurrStatus, newStatus, false) {
			logger.Trace(
				"resource's status is unchanged",
				"conditions", newStatus.Conditions,
			)
		} else {
			_, err := rt.Client.WriteStatus(ctx, &pbresource.WriteStatusRequest{
				Id:     state.ID,
				Key:    StatusKey,
				Status: newStatus,
			})

			if err != nil {
				logger.Error(
					"error encountered when attempting to update the resource's status",
					"error", err,
				)
				return err
			}

			logger.Trace(
				"resource's status was updated",
				"conditions", newStatus.Conditions,
			)
		}
	}

	return nil
}
