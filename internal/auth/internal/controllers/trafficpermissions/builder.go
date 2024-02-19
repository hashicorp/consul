// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package trafficpermissions

import (
	"sort"

	"github.com/hashicorp/consul/internal/auth/internal/controllers/trafficpermissions/expander"
	"github.com/hashicorp/consul/internal/auth/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type trafficPermissionsBuilder struct {
	missing            map[resource.ReferenceKey]missingSamenessGroupReferences
	isDefault          bool
	allowedPermissions []*pbauth.Permission
	denyPermissions    []*pbauth.Permission
	sgExpander         expander.SamenessGroupExpander
	sgMap              map[string][]*pbmulticluster.SamenessGroupMember
	brc                *resource.BoundReferenceCollector
}

type missingSamenessGroupReferences struct {
	resource       *pbresource.Resource
	samenessGroups []string
}

func newTrafficPermissionsBuilder(
	expander expander.SamenessGroupExpander,
	sgMap map[string][]*pbmulticluster.SamenessGroupMember,
	brc *resource.BoundReferenceCollector) *trafficPermissionsBuilder {
	return &trafficPermissionsBuilder{
		sgMap:              sgMap,
		missing:            make(map[resource.ReferenceKey]missingSamenessGroupReferences),
		isDefault:          true,
		sgExpander:         expander,
		allowedPermissions: make([]*pbauth.Permission, 0),
		denyPermissions:    make([]*pbauth.Permission, 0),
		brc:                brc,
	}
}

// track will use all associated XTrafficPermissions to create new ComputedTrafficPermissions samenessGroupsForTrafficPermission
func track[S types.XTrafficPermissions](tpb *trafficPermissionsBuilder, xtp *resource.DecodedResource[S]) {
	tpb.brc.AddRefOrID(xtp.Id)

	permissions, missingSamenessGroups := tpb.sgExpander.Expand(xtp.Data, tpb.sgMap)

	if len(missingSamenessGroups) > 0 {
		tpb.missing[resource.NewReferenceKey(xtp.Id)] = missingSamenessGroupReferences{
			resource:       xtp.Resource,
			samenessGroups: missingSamenessGroups,
		}
	}

	tpb.isDefault = false

	if xtp.Data.GetAction() == pbauth.Action_ACTION_ALLOW {
		tpb.allowedPermissions = append(tpb.allowedPermissions, permissions...)
	} else {
		tpb.denyPermissions = append(tpb.denyPermissions, permissions...)
	}
}

func (tpb *trafficPermissionsBuilder) build() (*pbauth.ComputedTrafficPermissions, map[resource.ReferenceKey]missingSamenessGroupReferences) {
	return &pbauth.ComputedTrafficPermissions{
		AllowPermissions: tpb.allowedPermissions,
		DenyPermissions:  tpb.denyPermissions,
		IsDefault:        tpb.isDefault,
		BoundReferences:  tpb.brc.List(),
	}, tpb.missing
}

func missingForCTP(missing map[resource.ReferenceKey]missingSamenessGroupReferences) []string {
	m := make(map[string]struct{})

	for _, sgRefs := range missing {
		for _, sg := range sgRefs.samenessGroups {
			m[sg] = struct{}{}
		}
	}

	out := make([]string, 0, len(m))

	for sg := range m {
		out = append(out, sg)
	}

	sort.Strings(out)

	return out
}
