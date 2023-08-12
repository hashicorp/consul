// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package pbpeerstream

const (
	apiTypePrefix = "type.googleapis.com/"

	TypeURLExportedService        = apiTypePrefix + "hashicorp.consul.internal.peerstream.ExportedService"
	TypeURLExportedServiceList    = apiTypePrefix + "hashicorp.consul.internal.peerstream.ExportedServiceList"
	TypeURLPeeringTrustBundle     = apiTypePrefix + "hashicorp.consul.internal.peering.PeeringTrustBundle"
	TypeURLPeeringServerAddresses = apiTypePrefix + "hashicorp.consul.internal.peering.PeeringServerAddresses"
)

func KnownTypeURL(s string) bool {
	switch s {
	case TypeURLExportedService, TypeURLExportedServiceList, TypeURLPeeringTrustBundle, TypeURLPeeringServerAddresses:
		return true
	}
	return false
}
