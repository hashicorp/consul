package structs

type SignResponse struct {
	IssuedCert *IssuedCert
	Roots      *IndexedCARoots
	GossipKey  string
}
