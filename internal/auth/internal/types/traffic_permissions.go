// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type DecodedTrafficPermissions = resource.DecodedResource[*pbauth.TrafficPermissions]

func RegisterTrafficPermissions(r resource.Registry) {
	r.Register(resource.Registration{
		Type:  pbauth.TrafficPermissionsType,
		Proto: &pbauth.TrafficPermissions{},
		ACLs: &resource.ACLHooks{
			Read:  resource.DecodeAndAuthorizeRead(aclReadHookTrafficPermissions),
			Write: resource.DecodeAndAuthorizeWrite(aclWriteHookTrafficPermissions),
			List:  resource.NoOpACLListHook,
		},
		Validate: ValidateTrafficPermissions,
		Mutate:   MutateTrafficPermissions,
		Scope:    resource.ScopeNamespace,
	})
}

var MutateTrafficPermissions = resource.DecodeAndMutate(mutateTrafficPermissions)

func mutateTrafficPermissions(res *DecodedTrafficPermissions) (bool, error) {
	var changed bool

	for _, p := range res.Data.Permissions {
		for _, s := range p.Sources {
			if updated := normalizedTenancyForSource(s, res.Id.Tenancy); updated {
				changed = true
			}
		}
	}

	return changed, nil
}

func normalizedTenancyForSource(src *pbauth.Source, parentTenancy *pbresource.Tenancy) bool {
	var changed bool

	if t, c := defaultedSourceTenancy(src, parentTenancy); c {
		src.Partition = t.Partition
		// TODO(peering/v2) revisit default peer source
		// src.Peer = t.PeerName
		src.Peer = resource.DefaultPeerName
		src.Namespace = t.Namespace
		changed = true
	}

	for _, e := range src.Exclude {
		if t, c := defaultedSourceTenancy(e, parentTenancy); c {
			e.Partition = t.Partition
			// TODO(peering/v2) revisit default peer source
			// e.Peer = t.PeerName
			e.Peer = resource.DefaultPeerName
			e.Namespace = t.Namespace
			changed = true
		}
	}

	return changed
}

func defaultedSourceTenancy(s pbauth.SourceToSpiffe, parentTenancy *pbresource.Tenancy) (*pbresource.Tenancy, bool) {
	if !isLocalPeer(s.GetPeer()) {
		return nil, false
	}

	tenancy := pbauth.SourceToTenancy(s)

	// TODO(peering/v2) default peer name somehow
	// var peerChanged bool
	// tenancy.PeerName, peerChanged = firstNonEmptyString(tenancy.PeerName, parentTenancy.PeerName, resource.DefaultPeerName)

	var partitionChanged bool
	tenancy.Partition, partitionChanged = firstNonEmptyString(tenancy.Partition, parentTenancy.Partition, resource.DefaultPartitionName)

	var namespaceChanged bool
	if s.GetIdentityName() != "" {
		if tenancy.Partition == parentTenancy.Partition {
			tenancy.Namespace, namespaceChanged = firstNonEmptyString(tenancy.Namespace, parentTenancy.Namespace, resource.DefaultNamespaceName)
		} else {
			tenancy.Namespace, namespaceChanged = firstNonEmptyString(tenancy.Namespace, resource.DefaultNamespaceName, resource.DefaultNamespaceName)
		}
	}

	// TODO(peering/v2) take peer being changed into account
	return tenancy, partitionChanged || namespaceChanged // || peerChange
}

func firstNonEmptyString(a, b, c string) (string, bool) {
	if a != "" {
		return a, false
	}

	if b != "" {
		return b, true
	}

	return c, true
}

var ValidateTrafficPermissions = resource.DecodeAndValidate(validateTrafficPermissions)

// validator takes a traffic permission and ensures that it conforms to the actions allowed in
// either CE or Enterprise versions of Consul
type validator interface {
	ValidateAction(data interface{ GetAction() pbauth.Action }) error
}

func validateTrafficPermissions(res *DecodedTrafficPermissions) error {
	var merr error

	if err := validateAction(res.Data); err != nil {
		merr = multierror.Append(merr, err)
	}

	if res.Data.Destination == nil || (len(res.Data.Destination.IdentityName) == 0) {
		merr = multierror.Append(merr, resource.ErrInvalidField{
			Name:    "data.destination",
			Wrapped: resource.ErrEmpty,
		})
	}
	// Validate permissions
	if err := validatePermissions(res.Id, res.Data); err != nil {
		merr = multierror.Append(merr, err)
	}

	return merr
}

func aclReadHookTrafficPermissions(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, res *DecodedTrafficPermissions) error {
	return authorizer.ToAllowAuthorizer().TrafficPermissionsReadAllowed(res.Data.Destination.IdentityName, authzContext)
}

func aclWriteHookTrafficPermissions(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, res *DecodedTrafficPermissions) error {
	return authorizer.ToAllowAuthorizer().TrafficPermissionsWriteAllowed(res.Data.Destination.IdentityName, authzContext)
}
