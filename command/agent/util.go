package agent

import (
	"math"
	"math/rand"
	"os/exec"
	"runtime"
	"time"
)

const (
	// This scale factor means we will add a minute after we
	// cross 128 nodes, another at 256, another at 512, etc.
	// By 8192 nodes, we will scale up by a factor of 8
	aeScaleThreshold = 128
)

// aeScale is used to scale the time interval at which anti-entropy
// take place. It is used to prevent saturation as the cluster size grows
func aeScale(interval time.Duration, n int) time.Duration {
	// Don't scale until we cross the threshold
	if n <= aeScaleThreshold {
		return interval
	}

	multiplier := math.Ceil(math.Log2(float64(n))-math.Log2(aeScaleThreshold)) + 1.0
	return time.Duration(multiplier) * interval
}

// Returns a random stagger interval between 0 and the duration
func randomStagger(intv time.Duration) time.Duration {
	return time.Duration(uint64(rand.Int63()) % uint64(intv))
}

// strContains checks if a list contains a string
func strContains(l []string, s string) bool {
	for _, v := range l {
		if v == s {
			return true
		}
	}
	return false
}

// ExecScript returns a command to execute a script
func ExecScript(script string) (*exec.Cmd, error) {
	var shell, flag string
	if runtime.GOOS == "windows" {
		shell = "cmd"
		flag = "/C"
	} else {
		shell = "/bin/sh"
		flag = "-c"
	}
	cmd := exec.Command(shell, flag, script)
	return cmd, nil
}
