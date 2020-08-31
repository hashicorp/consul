package acl

const (
	WildcardName = "*"
)

// Config encapsulates all of the generic configuration parameters used for
// policy parsing and enforcement
type Config struct {
	// WildcardName is the string that represents a request to authorize a wildcard permission
	WildcardName string

	// embedded enterprise configuration
	EnterpriseConfig
}

// GetWildcardName will retrieve the configured wildcard name or provide a default
// in the case that the config is Nil or the wildcard name is unset.
func (c *Config) GetWildcardName() string {
	if c == nil || c.WildcardName == "" {
		return WildcardName
	}
	return c.WildcardName
}

// Close will relinquish any resources this Config might be holding on to or
// managing.
func (c *Config) Close() {
	if c != nil {
		c.EnterpriseConfig.Close()
	}
}
