package net

import (
	"os"
	"syscall"
	"testing"

	"github.com/shirou/gopsutil/internal/common"
	"github.com/stretchr/testify/assert"
)

func TestGetProcInodesAll(t *testing.T) {
	if os.Getenv("CIRCLECI") == "true" {
		t.Skip("Skip CI")
	}

	root := common.HostProc("")
	v, err := getProcInodesAll(root, 0)
	assert.Nil(t, err)
	assert.NotEmpty(t, v)
}

func TestConnectionsMax(t *testing.T) {
	if os.Getenv("CIRCLECI") == "true" {
		t.Skip("Skip CI")
	}

	max := 10
	v, err := ConnectionsMax("tcp", max)
	assert.Nil(t, err)
	assert.NotEmpty(t, v)

	cxByPid := map[int32]int{}
	for _, cx := range v {
		if cx.Pid > 0 {
			cxByPid[cx.Pid]++
		}
	}
	for _, c := range cxByPid {
		assert.True(t, c <= max)
	}
}

type AddrTest struct {
	IP    string
	Port  int
	Error bool
}

func TestDecodeAddress(t *testing.T) {
	assert := assert.New(t)

	addr := map[string]AddrTest{
		"0500000A:0016": AddrTest{
			IP:   "10.0.0.5",
			Port: 22,
		},
		"0100007F:D1C2": AddrTest{
			IP:   "127.0.0.1",
			Port: 53698,
		},
		"11111:0035": AddrTest{
			Error: true,
		},
		"0100007F:BLAH": AddrTest{
			Error: true,
		},
		"0085002452100113070057A13F025401:0035": AddrTest{
			IP:   "2400:8500:1301:1052:a157:7:154:23f",
			Port: 53,
		},
		"00855210011307F025401:0035": AddrTest{
			Error: true,
		},
	}

	for src, dst := range addr {
		family := syscall.AF_INET
		if len(src) > 13 {
			family = syscall.AF_INET6
		}
		addr, err := decodeAddress(uint32(family), src)
		if dst.Error {
			assert.NotNil(err, src)
		} else {
			assert.Nil(err, src)
			assert.Equal(dst.IP, addr.IP, src)
			assert.Equal(dst.Port, int(addr.Port), src)
		}
	}
}

func TestReverse(t *testing.T) {
	src := []byte{0x01, 0x02, 0x03}
	assert.Equal(t, []byte{0x03, 0x02, 0x01}, Reverse(src))
}
