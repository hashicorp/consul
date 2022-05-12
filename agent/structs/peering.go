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
