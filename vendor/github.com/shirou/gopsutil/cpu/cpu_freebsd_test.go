package cpu

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/shirou/gopsutil/internal/common"
)

func TestParseDmesgBoot(t *testing.T) {
	if runtime.GOOS != "freebsd" {
		t.SkipNow()
	}

	var cpuTests = []struct {
		file   string
		cpuNum int
		cores  int32
	}{
		{"1cpu_2core.txt", 1, 2},
		{"1cpu_4core.txt", 1, 4},
		{"2cpu_4core.txt", 2, 4},
	}
	for _, tt := range cpuTests {
		v, num, err := parseDmesgBoot(filepath.Join("expected/freebsd/", tt.file))
		if err != nil {
			t.Errorf("parseDmesgBoot failed(%s), %v", tt.file, err)
		}
		if num != tt.cpuNum {
			t.Errorf("parseDmesgBoot wrong length(%s), %v", tt.file, err)
		}
		if v.Cores != tt.cores {
			t.Errorf("parseDmesgBoot wrong core(%s), %v", tt.file, err)
		}
		if !common.StringsContains(v.Flags, "fpu") {
			t.Errorf("parseDmesgBoot fail to parse features(%s), %v", tt.file, err)
		}
	}
}
