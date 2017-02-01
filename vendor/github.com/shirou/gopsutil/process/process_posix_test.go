// +build linux freebsd

package process

import (
	"os"
	"syscall"
	"testing"
)

func Test_SendSignal(t *testing.T) {
	checkPid := os.Getpid()

	p, _ := NewProcess(int32(checkPid))
	err := p.SendSignal(syscall.SIGCONT)
	if err != nil {
		t.Errorf("send signal %v", err)
	}
}
