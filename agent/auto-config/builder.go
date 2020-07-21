package autoconf

import (
	"github.com/hashicorp/consul/agent/config"
)

// LoadConfig will build the configuration including the extraHead source injected
// after all other defaults but before any user supplied configuration and the overrides
// source injected as the final source in the configuration parsing chain.
func LoadConfig(builderOpts config.BuilderOpts, extraHead config.Source, overrides ...config.Source) (*config.RuntimeConfig, []string, error) {
	b, err := config.NewBuilder(builderOpts)
	if err != nil {
		return nil, nil, err
	}

	if extraHead.Data != "" {
		b.Head = append(b.Head, extraHead)
	}

	if len(overrides) != 0 {
		b.Tail = append(b.Tail, overrides...)
	}

	cfg, err := b.BuildAndValidate()
	if err != nil {
		return nil, nil, err
	}

	return &cfg, b.Warnings, nil
}
