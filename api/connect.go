package api

// Connect can be used to work with endpoints related to Connect, the
// feature for securely connecting services within Consul.
type Connect struct {
	c *Client
}

// Health returns a handle to the health endpoints
func (c *Client) Connect() *Connect {
	return &Connect{c}
}
