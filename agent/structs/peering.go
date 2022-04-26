package structs

// PeeringToken identifies a peer in order for a connection to be established.
type PeeringToken struct {
	CA              []string
	ServerAddresses []string
	ServerName      string
	PeerID          string
}
