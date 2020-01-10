package acl

const (
	WildcardName = "*"
)

type Config struct {
	// WildcardName is the string that represents a request to authorize a wildcard permission
	WildcardName string

	// embedded enterprise configuration
	EnterpriseConfig
}

func (c *Config) GetWildcardName() string {
	if c == nil || c.WildcardName == "" {
		return WildcardName
	}
	return c.WildcardName
}

func (c *Config) Close() {
	if c != nil {
		c.EnterpriseConfig.Close()
	}
}
