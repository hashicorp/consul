package debug

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCollectHostInfo(t *testing.T) {
	assert := assert.New(t)

	host := CollectHostInfo()

	assert.Nil(host.Errors)

	assert.NotNil(host.CollectionTime)
	assert.NotNil(host.Host)
	assert.NotNil(host.Disk)
	assert.NotNil(host.Memory)
}
