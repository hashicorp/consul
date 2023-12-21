package debug

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCollectHostInfo(t *testing.T) {

	host := CollectHostInfo()

	assert.Nil(t, host.Errors)

	assert.NotNil(t, host.CollectionTime)
	assert.NotNil(t, host.Host)
	assert.NotNil(t, host.Disk)
	assert.NotNil(t, host.Memory)
}
