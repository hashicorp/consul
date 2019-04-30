package structs

type SignResponse struct {
	IssuedCert           IssuedCert     `json:",omitempty"`
	ConnectCARoots       IndexedCARoots `json:",omitempty"`
	ManualCARoots        []string       `json:",omitempty"`
	GossipKey            string         `json:",omitempty"`
	VerifyServerHostname bool           `json:",omitempty"`
}
