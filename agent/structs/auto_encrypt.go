package structs

type SignResponse struct {
	CertPEM  string `json:",omitempty"`
	Agent    string `json:",omitempty"`
	AgentURI string `json:",omitempty"`
	RootCAs  string `json:",omitempty"`
}
