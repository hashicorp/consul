package structs

type SignResponse struct {
	Agent                string   `json:",omitempty"`
	AgentURI             string   `json:",omitempty"`
	CertPEM              string   `json:",omitempty"`
	RootCAs              []string `json:",omitempty"`
	VerifyServerHostname bool     `json:",omitempty"`
}
