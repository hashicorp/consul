package retry

import (
	"runtime"
	"strings"
	"time"
)

// Retryer retries a function until it returns nil
// or until a timeout has elapsed.
type Retryer struct {
	Timeout time.Duration
	Wait    time.Duration
}

// Retry retries f until it returns nil. f is retried until the
// timeout occurrs in which case the last error is returned.
func (r *Retryer) Retry(f func() error) (err error) {
	stop := time.Now().Add(r.Timeout)
	for {
		err = f()
		if err == nil || time.Now().After(stop) {
			return
		}
		time.Sleep(r.Wait)
	}
}

const (
	// timeout is the default time to wait for retries to complete
	// without error.
	timeout = time.Second

	// wait is the default time to wait between retries.
	wait = 25 * time.Millisecond
)

// Func retries f until it returns nil. f is retried until the default
// timeout occurrs in which case the last error is returned.
func Func(f func() error) (err error) {
	return FuncTimeout(timeout, f)
}

// FuncTimeout retries f until it returns nil. f is retried until the given
// timeout occurrs in which case the last error is returned.
func FuncTimeout(d time.Duration, f func() error) (err error) {
	r := &Retryer{Timeout: d, Wait: wait}
	return r.Retry(f)
}

// Fataler defines an interface compatible with testing.T
type Fataler interface {
	Fatalf(format string, args ...interface{})
}

// Fatal retries f until it returns nil. f is retried until the default
// timeout occurrs in which case t.Fatal is called with the last error.
func Fatal(t Fataler, f func() error) {
	fatal(t, timeout, f)
}

// FatalTimeout retries f until it returns nil. f is retried until the given
// timeout occurrs in which case t.Fatal is called with the last error.
func FatalTimeout(t Fataler, d time.Duration, f func() error) {
	fatal(t, d, f)
}

func fatal(t Fataler, d time.Duration, f func() error) {
	if err := FuncTimeout(d, f); err != nil {
		_, file, line, _ := runtime.Caller(2)
		t.Fatalf("%s:%d: %s", hcpath(file), line, err)
	}
}

// hcpath returns the relative path after github.com/hashicorp/consul/...
func hcpath(p string) string {
	path := strings.Split(p, "/")
	for i := len(path) - 1; i > 0; i-- {
		if path[i] == "hashicorp" {
			return strings.Join(path[i+2:], "/")
		}
	}
	return p
}
