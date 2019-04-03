package structs

type SignResponse struct {
	IssuedCert   *IssuedCert
	ConnectRoots *IndexedCARoots
	ManualRoots  [][]byte
	GossipKey    string
}
