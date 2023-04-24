package pbpeerstream

const (
	apiTypePrefix = "type.googleapis.com/"

	TypeURLExportedService        = apiTypePrefix + "hashicorp.consul.internal.peerstream.ExportedService"
	TypeURLPeeringTrustBundle     = apiTypePrefix + "hashicorp.consul.internal.peering.PeeringTrustBundle"
	TypeURLPeeringServerAddresses = apiTypePrefix + "hashicorp.consul.internal.peering.PeeringServerAddresses"
)

func KnownTypeURL(s string) bool {
	switch s {
	case TypeURLExportedService, TypeURLPeeringTrustBundle, TypeURLPeeringServerAddresses:
		return true
	}
	return false
}
