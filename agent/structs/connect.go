package structs

import "github.com/hashicorp/consul/acl"

// ConnectAuthorizeRequest is the structure of a request to authorize
// a connection.
type ConnectAuthorizeRequest struct {
	// Target is the name of the service that is being requested.
	Target string

	// EnterpriseMeta is the embedded Consul Enterprise specific metadata
	acl.EnterpriseMeta

	// ClientCertURI is a unique identifier for the requesting client. This
	// is currently the URI SAN from the TLS client certificate.
	//
	// ClientCertSerial is a colon-hex-encoded of the serial number for
	// the requesting client cert. This is used to check against revocation
	// lists.
	ClientCertURI    string
	ClientCertSerial string
}

func (req *ConnectAuthorizeRequest) TargetPartition() string {
	return req.PartitionOrDefault()
}
