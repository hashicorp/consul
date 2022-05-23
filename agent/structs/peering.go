package structs

// PeeringToken identifies a peer in order for a connection to be established.
type PeeringToken struct {
	CA              []string
	ServerAddresses []string
	ServerName      string
	PeerID          string
}

// PeeredService is a service that has been configured with an exported-service config entry to be exported to a peer.
type PeeredService struct {
	Name     ServiceName
	PeerName string
}

// NOTE: this is not serialized via msgpack so it can be changed without concern.
type ExportedServiceList struct {
	// Services is a list of exported services that apply to both standard
	// service discovery and service mesh.
	Services []ServiceName

	// DiscoChains is a list of exported service that ONLY apply to service mesh.
	DiscoChains []ServiceName
}

// ListAllDiscoveryChains returns all discovery chains (union of Services and
// DiscoChains).
func (list *ExportedServiceList) ListAllDiscoveryChains() map[ServiceName]struct{} {
	chainsByName := make(map[ServiceName]struct{})
	if list == nil {
		return chainsByName
	}

	for _, svc := range list.Services {
		chainsByName[svc] = struct{}{}
	}
	for _, chainName := range list.DiscoChains {
		chainsByName[chainName] = struct{}{}
	}
	return chainsByName
}
