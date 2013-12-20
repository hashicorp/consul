package agent

// Config is the configuration that can be set for an Agent.
// Some of this is configurable as CLI flags, but most must
// be set using a configuration file.
type Config struct {
}

// DefaultConfig is used to return a sane default configuration
func DefaultConfig() *Config {
	return &Config{}
}
