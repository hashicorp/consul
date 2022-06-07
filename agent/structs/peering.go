package structs

// PeeringToken identifies a peer in order for a connection to be established.
type PeeringToken struct {
	CA              []string
	ServerAddresses []string
	ServerName      string
	PeerID          string
}

type IndexedExportedServiceList struct {
	Services map[string]ServiceList
	QueryMeta
}

// NOTE: this is not serialized via msgpack so it can be changed without concern.
type ExportedServiceList struct {
	// Services is a list of exported services that apply to both standard
	// service discovery and service mesh.
	Services []ServiceName

	// DiscoChains is a list of exported service that ONLY apply to service mesh.
	DiscoChains []ServiceName

	// TODO(peering): reduce duplication here in the response
	ConnectProtocol map[ServiceName]string
}

// ListAllDiscoveryChains returns all discovery chains (union of Services and
// DiscoChains).
func (list *ExportedServiceList) ListAllDiscoveryChains() map[ServiceName]string {
	chainsByName := make(map[ServiceName]string)
	if list == nil {
		return chainsByName
	}

	for _, svc := range list.Services {
		chainsByName[svc] = list.ConnectProtocol[svc]
	}
	for _, chainName := range list.DiscoChains {
		chainsByName[chainName] = list.ConnectProtocol[chainName]
	}
	return chainsByName
}
