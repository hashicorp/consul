package agent

import (
	"bytes"
	"crypto/md5"
	crand "crypto/rand"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strconv"
	"time"

	"github.com/hashicorp/go-msgpack/codec"
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
	if other := os.Getenv("SHELL"); other != "" {
		shell = other
	}
	cmd := exec.Command(shell, flag, script)
	return cmd, nil
}

// generateUUID is used to generate a random UUID
func generateUUID() string {
	buf := make([]byte, 16)
	if _, err := crand.Read(buf); err != nil {
		panic(fmt.Errorf("failed to read random bytes: %v", err))
	}

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%12x",
		buf[0:4],
		buf[4:6],
		buf[6:8],
		buf[8:10],
		buf[10:16])
}

// decodeMsgPack is used to decode a MsgPack encoded object
func decodeMsgPack(buf []byte, out interface{}) error {
	return codec.NewDecoder(bytes.NewReader(buf), msgpackHandle).Decode(out)
}

// encodeMsgPack is used to encode an object with msgpack
func encodeMsgPack(msg interface{}) ([]byte, error) {
	var buf bytes.Buffer
	err := codec.NewEncoder(&buf, msgpackHandle).Encode(msg)
	return buf.Bytes(), err
}

// stringHash returns a simple md5sum for a string.
func stringHash(s string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(s)))
}

// FilePermissions is an interface which allows a struct to set
// ownership and permissions easily on a file it describes.
type FilePermissions interface {
	// User returns a user ID or user name
	User() string

	// Group returns a group ID. Group names are not supported.
	Group() string

	// Mode returns a string of file mode bits e.g. "0644"
	Mode() string
}

// setFilePermissions handles configuring ownership and permissions settings
// on a given file. It takes a path and any struct implementing the
// FilePermissions interface. All permission/ownership settings are optional.
// If no user or group is specified, the current user/group will be used. Mode
// is optional, and has no default (the operation is not performed if absent).
// User may be specified by name or ID, but group may only be specified by ID.
func setFilePermissions(path string, p FilePermissions) error {
	var err error
	uid, gid := os.Getuid(), os.Getgid()

	if p.User() != "" {
		if uid, err = strconv.Atoi(p.User()); err == nil {
			goto GROUP
		}

		// Try looking up the user by name
		if u, err := user.Lookup(p.User()); err == nil {
			uid, _ = strconv.Atoi(u.Uid)
			goto GROUP
		}

		return fmt.Errorf("invalid user specified: %v", p.User())
	}

GROUP:
	if p.Group() != "" {
		if gid, err = strconv.Atoi(p.Group()); err != nil {
			return fmt.Errorf("invalid group specified: %v", p.Group())
		}
	}
	if err := os.Chown(path, uid, gid); err != nil {
		return fmt.Errorf("failed setting ownership to %d:%d on %q: %s",
			uid, gid, path, err)
	}

	if p.Mode() != "" {
		mode, err := strconv.ParseUint(p.Mode(), 8, 32)
		if err != nil {
			return fmt.Errorf("invalid mode specified: %v", p.Mode())
		}
		if err := os.Chmod(path, os.FileMode(mode)); err != nil {
			return fmt.Errorf("failed setting permissions to %d on %q: %s",
				mode, path, err)
		}
	}

	return nil
}
