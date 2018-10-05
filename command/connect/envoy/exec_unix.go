// +build linux darwin

package envoy

import (
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"golang.org/x/sys/unix"
)

func execEnvoy(binary string, prefixArgs, suffixArgs []string, bootstrapJson []byte) error {
	// Write the Envoy bootstrap config file out to disk in a pocket universe
	// visible only to the current process (and exec'd future selves).
	fd, err := writeEphemeralEnvoyTempFile(bootstrapJson)
	if err != nil {
		return errors.New("Could not write envoy bootstrap config to a temp file: " + err.Error())
	}

	// On unix systems after exec the file descriptors that we should see:
	//
	//   0: stdin
	//   1: stdout
	//   2: stderr
	//   ... any open file descriptors from the parent without CLOEXEC set
	//
	// Above we explicitly disabled CLOEXEC for our temp file, so assuming
	// FD numbers survive across execs, it should just be the value of
	// `fd`. This is accessible as a file itself (trippy!) under
	// /dev/fd/$FDNUMBER.
	magicPath := filepath.Join("/dev/fd", strconv.Itoa(int(fd)))

	// First argument needs to be the executable name.
	envoyArgs := []string{binary}
	envoyArgs = append(envoyArgs, prefixArgs...)
	envoyArgs = append(envoyArgs, "--v2-config-only",
		"--disable-hot-restart",
		"--config-path",
		magicPath,
	)
	envoyArgs = append(envoyArgs, suffixArgs...)

	// Exec
	if err = unix.Exec(binary, envoyArgs, os.Environ()); err != nil {
		return errors.New("Failed to exec envoy: " + err.Error())
	}

	return nil
}

func writeEphemeralEnvoyTempFile(b []byte) (uintptr, error) {
	f, err := ioutil.TempFile("", "envoy-ephemeral-config")
	if err != nil {
		return 0, err
	}

	errFn := func(err error) (uintptr, error) {
		_ = f.Close()
		return 0, err
	}

	// TempFile already does this, but it's cheap to reinforce that we
	// WANT the default behavior.
	if err := f.Chmod(0600); err != nil {
		return errFn(err)
	}

	// Immediately unlink the file as we are going to just pass the
	// file descriptor, not the path.
	if err = os.Remove(f.Name()); err != nil {
		return errFn(err)
	}
	if _, err = f.Write(b); err != nil {
		return errFn(err)
	}
	// Rewind the file descriptor so Envoy can read it.
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return errFn(err)
	}

	// Disable CLOEXEC so that this file descriptor is available
	// to the exec'd Envoy.
	if err := setCloseOnExec(f.Fd(), false); err != nil {
		return errFn(err)
	}

	return f.Fd(), nil
}

// isCloseOnExec checks the provided file descriptor to see if the CLOEXEC flag
// is set.
func isCloseOnExec(fd uintptr) (bool, error) {
	flags, err := getFdFlags(fd)
	if err != nil {
		return false, err
	}
	return flags&unix.FD_CLOEXEC != 0, nil
}

// setCloseOnExec sets or unsets the CLOEXEC flag on the provided file descriptor
// depending upon the value of the enabled arg.
func setCloseOnExec(fd uintptr, enabled bool) error {
	flags, err := getFdFlags(fd)
	if err != nil {
		return err
	}

	newFlags := flags
	if enabled {
		newFlags |= unix.FD_CLOEXEC
	} else {
		newFlags &= ^unix.FD_CLOEXEC
	}

	if newFlags == flags {
		return nil // noop
	}

	_, err = unix.FcntlInt(fd, unix.F_SETFD, newFlags)
	return err
}

func getFdFlags(fd uintptr) (int, error) {
	return unix.FcntlInt(fd, unix.F_GETFD, 0)
}
