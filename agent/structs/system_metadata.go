package structs

// SystemMetadataOp is the operation for a request related to system metadata.
type SystemMetadataOp string

const (
	SystemMetadataUpsert SystemMetadataOp = "upsert"
	SystemMetadataDelete SystemMetadataOp = "delete"
)

// SystemMetadataRequest is used to upsert and delete system metadata.
type SystemMetadataRequest struct {
	// Datacenter is the target for this request.
	Datacenter string

	// Op is the type of operation being requested.
	Op SystemMetadataOp

	// Entry is the key to modify.
	Entry *SystemMetadataEntry

	// WriteRequest is a common struct containing ACL tokens and other
	// write-related common elements for requests.
	WriteRequest
}

const (
	SystemMetadataIntentionFormatKey           = "intention-format"
	SystemMetadataIntentionFormatConfigValue   = "config-entry"
	SystemMetadataIntentionFormatLegacyValue   = "legacy"
	SystemMetadataVirtualIPsEnabled            = "virtual-ips"
	SystemMetadataTermGatewayVirtualIPsEnabled = "virtual-ips-term-gateway"
)

type SystemMetadataEntry struct {
	Key   string
	Value string `json:",omitempty"`
	RaftIndex
}

// RequestDatacenter returns the datacenter for a given request.
func (c *SystemMetadataRequest) RequestDatacenter() string {
	return c.Datacenter
}
