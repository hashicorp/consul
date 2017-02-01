// +build darwin

package mem

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVirtualMemoryDarwin(t *testing.T) {
	v, err := VirtualMemory()
	assert.Nil(t, err)

	outBytes, err := invoke.Command("/usr/sbin/sysctl", "hw.memsize")
	assert.Nil(t, err)
	outString := string(outBytes)
	outString = strings.TrimSpace(outString)
	outParts := strings.Split(outString, " ")
	actualTotal, err := strconv.ParseInt(outParts[1], 10, 64)
	assert.Nil(t, err)
	assert.Equal(t, uint64(actualTotal), v.Total)

	assert.True(t, v.Available > 0)
	assert.Equal(t, v.Available, v.Free+v.Inactive, "%v", v)

	assert.True(t, v.Used > 0)
	assert.True(t, v.Used < v.Total)

	assert.True(t, v.UsedPercent > 0)
	assert.True(t, v.UsedPercent < 100)

	assert.True(t, v.Free > 0)
	assert.True(t, v.Free < v.Available)

	assert.True(t, v.Active > 0)
	assert.True(t, v.Active < v.Total)

	assert.True(t, v.Inactive > 0)
	assert.True(t, v.Inactive < v.Total)

	assert.True(t, v.Wired > 0)
	assert.True(t, v.Wired < v.Total)
}
