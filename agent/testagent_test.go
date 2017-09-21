package agent

import (
	"testing"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/hcl"
)

func TestDefaultConfig(t *testing.T) {
	for i := 0; i < 500; i++ {
		t.Run("bla", func(t *testing.T) {
			t.Parallel()
			var c config.Config
			data := config.DefaultSource().Data
			hcl.Decode(&c, data)
			hcl.Decode(&c, data)
			hcl.Decode(&c, data)
			hcl.Decode(&c, data)
			hcl.Decode(&c, data)
			hcl.Decode(&c, data)
			hcl.Decode(&c, data)
			hcl.Decode(&c, data)
			hcl.Decode(&c, data)
		})
	}
}
