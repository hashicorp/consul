package structs

import (
	"github.com/hashicorp/consul/api"
)

// EnvoyExtension has configuration for an extension that patches Envoy resources.
type EnvoyExtension struct {
	Name      string
	Required  bool
	Arguments map[string]interface{} `bexpr:"-"`
}

type EnvoyExtensions []EnvoyExtension

func (es EnvoyExtensions) ToAPI() []api.EnvoyExtension {
	extensions := make([]api.EnvoyExtension, len(es))
	for i, e := range es {
		extensions[i] = api.EnvoyExtension{
			Name:      e.Name,
			Required:  e.Required,
			Arguments: e.Arguments,
		}
	}
	return extensions
}
