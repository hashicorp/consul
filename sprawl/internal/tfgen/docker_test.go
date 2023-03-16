package tfgen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDockerImageResourceName(t *testing.T) {
	fn := DockerImageResourceName

	assert.Equal(t, "", fn(""))
	assert.Equal(t, "abcdefghijklmnopqrstuvwxyz0123456789-", fn("abcdefghijklmnopqrstuvwxyz0123456789-"))
	assert.Equal(t, "hashicorp-consul-1-15-0", fn("hashicorp/consul:1.15.0"))
}
