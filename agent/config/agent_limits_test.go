package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildAndValidate_HTTPMaxConnsPerClientExceedsRLimit(t *testing.T) {
	hcl := `
		limits{
			# We put a very high value to be sure to fail
			# This value is more than max on Windows as well
			http_max_conns_per_client = 16777217
		}`
	b, err := NewBuilder(BuilderOpts{})
	assert.NoError(t, err)
	testsrc := FileSource{
		Name:   "test",
		Format: "hcl",
		Data: `
		    ae_interval = "1m"
		    data_dir="/tmp/00000000001979"
			bind_addr = "127.0.0.1"
			advertise_addr = "127.0.0.1"
			datacenter = "dc1"
			bootstrap = true
			server = true
			node_id = "00000000001979"
			node_name = "Node-00000000001979"
		`,
	}
	b.Head = append(b.Head, testsrc)
	b.Tail = append(b.Tail, DefaultConsulSource(), DevConsulSource())
	b.Tail = append(b.Head, FileSource{Name: "hcl", Format: "hcl", Data: hcl})

	_, validationError := b.BuildAndValidate()
	if validationError == nil {
		assert.Fail(t, "Config should not be valid")
	}
	assert.Contains(t, validationError.Error(), "but limits.http_max_conns_per_client: 16777217 needs at least 16777237")
}
