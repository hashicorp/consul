package structs

type SignResponse struct {
	IssuedCert *IssuedCert
	ActiveRoot CARoot
	GossipKey  string
}
