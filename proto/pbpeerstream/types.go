package pbpeerstream

const (
	apiTypePrefix = "type.googleapis.com/"

	TypeURLExportedService    = apiTypePrefix + "hashicorp.consul.internal.peerstream.ExportedService"
	TypeURLPeeringTrustBundle = apiTypePrefix + "hashicorp.consul.internal.peering.PeeringTrustBundle"
)

func KnownTypeURL(s string) bool {
	switch s {
	case TypeURLExportedService, TypeURLPeeringTrustBundle:
		return true
	}
	return false
}
