package structs

type SignResponse struct {
	IssuedCert   *IssuedCert
	ConnectRoots *IndexedCARoots
	ManualRoots  []string
	GossipKey    string
}
