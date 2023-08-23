package checks

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/hashicorp/consul/agent/mock"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

func uniqueID() string {
	id, err := uuid.GenerateUUID()
	if err != nil {
		panic(err)
	}
	return id
}

func TestCheckMonitor_Script(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	tests := []struct {
		script, status string
	}{
		{"exit 0", "passing"},
		{"exit 1", "warning"},
		{"exit 2", "critical"},
		{"foobarbaz", "critical"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			notif := mock.NewNotify()
			logger := testutil.Logger(t)
			statusHandler := NewStatusHandler(notif, logger, 0, 0, 0)

			cid := structs.NewCheckID("foo", nil)
			check := &CheckMonitor{
				Notify:        notif,
				CheckID:       cid,
				Script:        tt.script,
				Interval:      25 * time.Millisecond,
				OutputMaxSize: DefaultBufSize,
				Logger:        logger,
				StatusHandler: statusHandler,
			}
			check.Start()
			defer check.Stop()
			retry.Run(t, func(r *retry.R) {
				if got, want := notif.Updates(cid), 2; got < want {
					r.Fatalf("got %d updates want at least %d", got, want)
				}
				if got, want := notif.State(cid), tt.status; got != want {
					r.Fatalf("got state %q want %q", got, want)
				}
			})
		})
	}
}

func TestCheckMonitor_Args(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	tests := []struct {
		args   []string
		status string
	}{
		{[]string{"sh", "-c", "exit 0"}, "passing"},
		{[]string{"sh", "-c", "exit 1"}, "warning"},
		{[]string{"sh", "-c", "exit 2"}, "critical"},
		{[]string{"foobarbaz"}, "critical"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			notif := mock.NewNotify()
			logger := testutil.Logger(t)
			statusHandler := NewStatusHandler(notif, logger, 0, 0, 0)
			cid := structs.NewCheckID("foo", nil)

			check := &CheckMonitor{
				Notify:        notif,
				CheckID:       cid,
				ScriptArgs:    tt.args,
				Interval:      25 * time.Millisecond,
				OutputMaxSize: DefaultBufSize,
				Logger:        logger,
				StatusHandler: statusHandler,
			}
			check.Start()
			defer check.Stop()
			retry.Run(t, func(r *retry.R) {
				if got, want := notif.Updates(cid), 2; got < want {
					r.Fatalf("got %d updates want at least %d", got, want)
				}
				if got, want := notif.State(cid), tt.status; got != want {
					r.Fatalf("got state %q want %q", got, want)
				}
			})
		})
	}
}

func TestCheckMonitor_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// t.Parallel() // timing test. no parallel
	notif := mock.NewNotify()
	logger := testutil.Logger(t)
	statusHandler := NewStatusHandler(notif, logger, 0, 0, 0)

	cid := structs.NewCheckID("foo", nil)
	check := &CheckMonitor{
		Notify:        notif,
		CheckID:       cid,
		ScriptArgs:    []string{"sh", "-c", "sleep 1 && exit 0"},
		Interval:      50 * time.Millisecond,
		Timeout:       25 * time.Millisecond,
		OutputMaxSize: DefaultBufSize,
		Logger:        logger,
		StatusHandler: statusHandler,
	}
	check.Start()
	defer check.Stop()

	time.Sleep(250 * time.Millisecond)

	// Should have at least 2 updates
	if notif.Updates(cid) < 2 {
		t.Fatalf("should have at least 2 updates %v", notif.UpdatesMap())
	}
	if notif.State(cid) != "critical" {
		t.Fatalf("should be critical %v", notif.StateMap())
	}
}

func TestCheckMonitor_RandomStagger(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// t.Parallel() // timing test. no parallel
	notif := mock.NewNotify()
	logger := testutil.Logger(t)
	statusHandler := NewStatusHandler(notif, logger, 0, 0, 0)

	cid := structs.NewCheckID("foo", nil)

	check := &CheckMonitor{
		Notify:        notif,
		CheckID:       cid,
		ScriptArgs:    []string{"sh", "-c", "exit 0"},
		Interval:      25 * time.Millisecond,
		OutputMaxSize: DefaultBufSize,
		Logger:        logger,
		StatusHandler: statusHandler,
	}
	check.Start()
	defer check.Stop()

	time.Sleep(500 * time.Millisecond)

	// Should have at least 1 update
	if notif.Updates(cid) < 1 {
		t.Fatalf("should have 1 or more updates %v", notif.UpdatesMap())
	}

	if notif.State(cid) != api.HealthPassing {
		t.Fatalf("should be %v %v", api.HealthPassing, notif.StateMap())
	}
}

func TestCheckMonitor_LimitOutput(t *testing.T) {
	t.Parallel()
	notif := mock.NewNotify()
	logger := testutil.Logger(t)
	statusHandler := NewStatusHandler(notif, logger, 0, 0, 0)
	cid := structs.NewCheckID("foo", nil)

	check := &CheckMonitor{
		Notify:        notif,
		CheckID:       cid,
		ScriptArgs:    []string{"od", "-N", "81920", "/dev/urandom"},
		Interval:      25 * time.Millisecond,
		OutputMaxSize: DefaultBufSize,
		Logger:        logger,
		StatusHandler: statusHandler,
	}
	check.Start()
	defer check.Stop()

	time.Sleep(50 * time.Millisecond)

	// Allow for extra bytes for the truncation message
	if len(notif.Output(cid)) > DefaultBufSize+100 {
		t.Fatalf("output size is too long")
	}
}

func TestCheckTTL(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// t.Parallel() // timing test. no parallel
	notif := mock.NewNotify()
	logger := testutil.Logger(t)
	cid := structs.NewCheckID("foo", nil)

	check := &CheckTTL{
		Notify:  notif,
		CheckID: cid,
		TTL:     200 * time.Millisecond,
		Logger:  logger,
	}
	check.Start()
	defer check.Stop()

	time.Sleep(100 * time.Millisecond)
	check.SetStatus(api.HealthPassing, "test-output")

	if notif.Updates(cid) != 1 {
		t.Fatalf("should have 1 updates %v", notif.UpdatesMap())
	}

	if notif.State(cid) != api.HealthPassing {
		t.Fatalf("should be passing %v", notif.StateMap())
	}

	// Ensure we don't fail early
	time.Sleep(150 * time.Millisecond)
	if notif.Updates(cid) != 1 {
		t.Fatalf("should have 1 updates %v", notif.UpdatesMap())
	}

	// Wait for the TTL to expire
	time.Sleep(150 * time.Millisecond)

	if notif.Updates(cid) != 2 {
		t.Fatalf("should have 2 updates %v", notif.UpdatesMap())
	}

	if notif.State(cid) != api.HealthCritical {
		t.Fatalf("should be critical %v", notif.StateMap())
	}

	if !strings.Contains(notif.Output(cid), "test-output") {
		t.Fatalf("should have retained output %v", notif.OutputMap())
	}
}

func TestCheckHTTP(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	tests := []struct {
		desc   string
		code   int
		method string
		header http.Header
		status string
	}{
		// passing
		{code: 200, status: api.HealthPassing},
		{code: 201, status: api.HealthPassing},
		{code: 250, status: api.HealthPassing},
		{code: 299, status: api.HealthPassing},

		// warning
		{code: 429, status: api.HealthWarning},

		// critical
		{code: 300, status: api.HealthCritical},
		{code: 400, status: api.HealthCritical},
		{code: 500, status: api.HealthCritical},

		// custom method
		{desc: "custom method GET", code: 200, method: "GET", status: api.HealthPassing},
		{desc: "custom method POST", code: 200, header: http.Header{"Content-Length": []string{"0"}}, method: "POST", status: api.HealthPassing},
		{desc: "custom method abc", code: 200, method: "abc", status: api.HealthPassing},

		// custom header
		{desc: "custom header", code: 200, header: http.Header{"A": []string{"b", "c"}}, status: api.HealthPassing},
		{desc: "host header", code: 200, header: http.Header{"Host": []string{"a"}}, status: api.HealthPassing},
	}

	for _, tt := range tests {
		desc := tt.desc
		if desc == "" {
			desc = fmt.Sprintf("code %d -> status %s", tt.code, tt.status)
		}
		t.Run(desc, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.method != "" && tt.method != r.Method {
					w.WriteHeader(999)
					return
				}

				expectedHeader := http.Header{
					"Accept":          []string{"text/plain, text/*, */*"},
					"Accept-Encoding": []string{"gzip"},
					"Connection":      []string{"close"},
					"User-Agent":      []string{"Consul Health Check"},
				}
				for k, v := range tt.header {
					expectedHeader[k] = v
				}

				// the Host header is in r.Host and not in the headers
				host := expectedHeader.Get("Host")
				if host != "" && host != r.Host {
					w.WriteHeader(999)
					return
				}
				expectedHeader.Del("Host")

				if !reflect.DeepEqual(expectedHeader, r.Header) {
					w.WriteHeader(999)
					return
				}

				// Body larger than 4k limit
				body := bytes.Repeat([]byte{'a'}, 2*DefaultBufSize)
				w.WriteHeader(tt.code)
				w.Write(body)
			}))
			defer server.Close()

			notif := mock.NewNotify()
			logger := testutil.Logger(t)
			statusHandler := NewStatusHandler(notif, logger, 0, 0, 0)

			cid := structs.NewCheckID("foo", nil)

			check := &CheckHTTP{
				CheckID:       cid,
				HTTP:          server.URL,
				Method:        tt.method,
				Header:        tt.header,
				Interval:      10 * time.Millisecond,
				Logger:        logger,
				StatusHandler: statusHandler,
			}
			check.Start()
			defer check.Stop()

			retry.Run(t, func(r *retry.R) {
				if got, want := notif.Updates(cid), 2; got < want {
					r.Fatalf("got %d updates want at least %d", got, want)
				}
				if got, want := notif.State(cid), tt.status; got != want {
					r.Fatalf("got state %q want %q", got, want)
				}
				// Allow slightly more data than DefaultBufSize, for the header
				if n := len(notif.Output(cid)); n > (DefaultBufSize + 256) {
					r.Fatalf("output too long: %d (%d-byte limit)", n, DefaultBufSize)
				}
			})
		})
	}
}

func TestCheckHTTP_Proxied(t *testing.T) {
	t.Parallel()

	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Proxy Server")
	}))
	defer proxy.Close()

	notif := mock.NewNotify()

	logger := testutil.Logger(t)
	statusHandler := NewStatusHandler(notif, logger, 0, 0, 0)
	cid := structs.NewCheckID("foo", nil)

	check := &CheckHTTP{
		CheckID:       cid,
		HTTP:          "",
		Method:        "GET",
		OutputMaxSize: DefaultBufSize,
		Interval:      10 * time.Millisecond,
		Logger:        logger,
		ProxyHTTP:     proxy.URL,
		StatusHandler: statusHandler,
	}

	check.Start()
	defer check.Stop()

	// If ProxyHTTP is set, check() reqs should go to that address
	retry.Run(t, func(r *retry.R) {
		output := notif.Output(cid)
		if !strings.Contains(output, "Proxy Server") {
			r.Fatalf("c.ProxyHTTP server did not receive request, but should")
		}
	})
}

func TestCheckHTTP_NotProxied(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Original Server")
	}))
	defer server.Close()

	notif := mock.NewNotify()
	logger := testutil.Logger(t)
	statusHandler := NewStatusHandler(notif, logger, 0, 0, 0)
	cid := structs.NewCheckID("foo", nil)

	check := &CheckHTTP{
		CheckID:       cid,
		HTTP:          server.URL,
		Method:        "GET",
		OutputMaxSize: DefaultBufSize,
		Interval:      10 * time.Millisecond,
		Logger:        logger,
		ProxyHTTP:     "",
		StatusHandler: statusHandler,
	}
	check.Start()
	defer check.Stop()

	// If ProxyHTTP is not set, check() reqs should go to the address in CheckHTTP.HTTP
	retry.Run(t, func(r *retry.R) {
		output := notif.Output(cid)
		if !strings.Contains(output, "Original Server") {
			r.Fatalf("server did not receive request")
		}
	})
}

func TestCheckHTTP_DisableRedirects(t *testing.T) {
	t.Parallel()

	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "server1")
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.RedirectHandler(server1.URL, 301))
	defer server2.Close()

	notif := mock.NewNotify()
	logger := testutil.Logger(t)
	statusHandler := NewStatusHandler(notif, logger, 0, 0, 0)
	cid := structs.NewCheckID("foo", nil)

	check := &CheckHTTP{
		CheckID:          cid,
		HTTP:             server2.URL,
		Method:           "GET",
		OutputMaxSize:    DefaultBufSize,
		Interval:         10 * time.Millisecond,
		DisableRedirects: true,
		Logger:           logger,
		StatusHandler:    statusHandler,
	}
	check.Start()
	defer check.Stop()

	retry.Run(t, func(r *retry.R) {
		output := notif.Output(cid)
		if !strings.Contains(output, "Moved Permanently") {
			r.Fatalf("should have returned 301 body instead of redirecting")
		}
		if strings.Contains(output, "server1") {
			r.Fatalf("followed redirect")
		}
	})
}

func TestCheckHTTPTCP_BigTimeout(t *testing.T) {
	testCases := []struct {
		timeoutIn, intervalIn, timeoutWant time.Duration
	}{
		{
			timeoutIn:   31 * time.Second,
			intervalIn:  30 * time.Second,
			timeoutWant: 31 * time.Second,
		},
		{
			timeoutIn:   30 * time.Second,
			intervalIn:  30 * time.Second,
			timeoutWant: 30 * time.Second,
		},
		{
			timeoutIn:   29 * time.Second,
			intervalIn:  30 * time.Second,
			timeoutWant: 29 * time.Second,
		},
		{
			timeoutIn:   0 * time.Second,
			intervalIn:  10 * time.Second,
			timeoutWant: 10 * time.Second,
		},
		{
			timeoutIn:   0 * time.Second,
			intervalIn:  30 * time.Second,
			timeoutWant: 10 * time.Second,
		},
		{
			timeoutIn:   10 * time.Second,
			intervalIn:  30 * time.Second,
			timeoutWant: 10 * time.Second,
		},
		{
			timeoutIn:   9 * time.Second,
			intervalIn:  30 * time.Second,
			timeoutWant: 9 * time.Second,
		},
		{
			timeoutIn:   -1 * time.Second,
			intervalIn:  10 * time.Second,
			timeoutWant: 10 * time.Second,
		},
		{
			timeoutIn:   0 * time.Second,
			intervalIn:  5 * time.Second,
			timeoutWant: 10 * time.Second,
		},
	}

	for _, tc := range testCases {
		desc := fmt.Sprintf("timeoutIn: %v, intervalIn: %v", tc.timeoutIn, tc.intervalIn)
		t.Run(desc, func(t *testing.T) {
			checkHTTP := &CheckHTTP{
				Timeout:  tc.timeoutIn,
				Interval: tc.intervalIn,
			}
			checkHTTP.Start()
			defer checkHTTP.Stop()
			if checkHTTP.httpClient.Timeout != tc.timeoutWant {
				t.Fatalf("expected HTTP timeout to be %v, got %v", tc.timeoutWant, checkHTTP.httpClient.Timeout)
			}

			checkTCP := &CheckTCP{
				Timeout:  tc.timeoutIn,
				Interval: tc.intervalIn,
			}
			checkTCP.Start()
			defer checkTCP.Stop()
			if checkTCP.dialer.Timeout != tc.timeoutWant {
				t.Fatalf("expected TCP timeout to be %v, got %v", tc.timeoutWant, checkTCP.dialer.Timeout)
			}
		})

	}
}

func TestCheckMaxOutputSize(t *testing.T) {
	t.Parallel()
	timeout := 5 * time.Millisecond
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		body := bytes.Repeat([]byte{'x'}, 2*DefaultBufSize)
		writer.WriteHeader(200)
		writer.Write(body)
	}))
	defer server.Close()

	notif := mock.NewNotify()
	logger := testutil.Logger(t)
	maxOutputSize := 32
	cid := structs.NewCheckID("bar", nil)

	check := &CheckHTTP{
		CheckID:       cid,
		HTTP:          server.URL + "/v1/agent/self",
		Timeout:       timeout,
		Interval:      2 * time.Millisecond,
		Logger:        logger,
		OutputMaxSize: maxOutputSize,
		StatusHandler: NewStatusHandler(notif, logger, 0, 0, 0),
	}

	check.Start()
	defer check.Stop()
	retry.Run(t, func(r *retry.R) {
		if got, want := notif.Updates(cid), 2; got < want {
			r.Fatalf("got %d updates want at least %d", got, want)
		}
		if got, want := notif.State(cid), api.HealthPassing; got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
		if got, want := notif.Output(cid), "HTTP GET "+server.URL+"/v1/agent/self: 200 OK Output: "+strings.Repeat("x", maxOutputSize); got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
	})
}

func TestCheckHTTPTimeout(t *testing.T) {
	t.Parallel()
	timeout := 5 * time.Millisecond
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		time.Sleep(2 * timeout)
	}))
	defer server.Close()

	notif := mock.NewNotify()
	logger := testutil.Logger(t)
	statusHandler := NewStatusHandler(notif, logger, 0, 0, 0)

	cid := structs.NewCheckID("bar", nil)

	check := &CheckHTTP{
		CheckID:       cid,
		HTTP:          server.URL,
		Timeout:       timeout,
		Interval:      10 * time.Millisecond,
		Logger:        logger,
		StatusHandler: statusHandler,
	}

	check.Start()
	defer check.Stop()
	retry.Run(t, func(r *retry.R) {
		if got, want := notif.Updates(cid), 2; got < want {
			r.Fatalf("got %d updates want at least %d", got, want)
		}
		if got, want := notif.State(cid), api.HealthCritical; got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
	})
}

func TestCheckHTTPBody(t *testing.T) {
	t.Parallel()
	timeout := 5 * time.Millisecond

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			buf  bytes.Buffer
			body []byte
		)
		code := 200
		if _, err := buf.ReadFrom(r.Body); err != nil {
			code = 999
			body = []byte(err.Error())
		} else {
			body = buf.Bytes()
		}

		w.WriteHeader(code)
		w.Write(body)
	}))
	defer server.Close()

	tests := []struct {
		desc   string
		method string
		header http.Header
		body   string
	}{
		{desc: "get body", method: "GET", body: "hello world"},
		{desc: "post body", method: "POST", body: "hello world"},
		{desc: "post json body", header: http.Header{"Content-Type": []string{"application/json"}}, method: "POST", body: "{\"foo\":\"bar\"}"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			notif := mock.NewNotify()

			cid := structs.NewCheckID("checkbody", nil)
			logger := testutil.Logger(t)
			check := &CheckHTTP{
				CheckID:       cid,
				HTTP:          server.URL,
				Header:        tt.header,
				Method:        tt.method,
				Body:          tt.body,
				Timeout:       timeout,
				Interval:      2 * time.Millisecond,
				Logger:        logger,
				StatusHandler: NewStatusHandler(notif, logger, 0, 0, 0),
			}
			check.Start()
			defer check.Stop()

			retry.Run(t, func(r *retry.R) {
				if got, want := notif.Updates(cid), 2; got < want {
					r.Fatalf("got %d updates want at least %d", got, want)
				}
				if got, want := notif.State(cid), api.HealthPassing; got != want {
					r.Fatalf("got status %q want %q", got, want)
				}
				if got, want := notif.Output(cid), tt.body; !strings.HasSuffix(got, want) {
					r.Fatalf("got output %q want suffix %q", got, want)
				}
			})
		})
	}
}

func TestCheckHTTP_disablesKeepAlives(t *testing.T) {
	t.Parallel()
	notif := mock.NewNotify()
	logger := testutil.Logger(t)
	cid := structs.NewCheckID("foo", nil)

	check := &CheckHTTP{
		CheckID:       cid,
		HTTP:          "http://foo.bar/baz",
		Interval:      10 * time.Second,
		Logger:        logger,
		StatusHandler: NewStatusHandler(notif, logger, 0, 0, 0),
	}

	check.Start()
	defer check.Stop()

	if !check.httpClient.Transport.(*http.Transport).DisableKeepAlives {
		t.Fatalf("should have disabled keepalives")
	}
}

func largeBodyHandler(code int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Body larger than 4k limit
		body := bytes.Repeat([]byte{'a'}, 2*DefaultBufSize)
		w.WriteHeader(code)
		w.Write(body)
	})
}

func TestCheckHTTP_TLS_SkipVerify(t *testing.T) {
	t.Parallel()
	server := httptest.NewTLSServer(largeBodyHandler(200))
	defer server.Close()

	tlsConfig := &api.TLSConfig{
		InsecureSkipVerify: true,
	}
	tlsClientConfig, err := api.SetupTLSConfig(tlsConfig)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	notif := mock.NewNotify()
	logger := testutil.Logger(t)
	statusHandler := NewStatusHandler(notif, logger, 0, 0, 0)

	cid := structs.NewCheckID("skipverify_true", nil)
	check := &CheckHTTP{
		CheckID:         cid,
		HTTP:            server.URL,
		Interval:        25 * time.Millisecond,
		Logger:          logger,
		TLSClientConfig: tlsClientConfig,
		StatusHandler:   statusHandler,
	}

	check.Start()
	defer check.Stop()

	if !check.httpClient.Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify {
		t.Fatalf("should be true")
	}

	retry.Run(t, func(r *retry.R) {
		if got, want := notif.State(cid), api.HealthPassing; got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
	})
}

func TestCheckHTTP_TLS_BadVerify(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	server := httptest.NewTLSServer(largeBodyHandler(200))
	defer server.Close()

	tlsClientConfig, err := api.SetupTLSConfig(&api.TLSConfig{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	notif := mock.NewNotify()
	logger := testutil.Logger(t)
	statusHandler := NewStatusHandler(notif, logger, 0, 0, 0)
	cid := structs.NewCheckID("skipverify_false", nil)

	check := &CheckHTTP{
		CheckID:         cid,
		HTTP:            server.URL,
		Interval:        100 * time.Millisecond,
		Logger:          logger,
		TLSClientConfig: tlsClientConfig,
		StatusHandler:   statusHandler,
	}

	check.Start()
	defer check.Stop()

	if check.httpClient.Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify {
		t.Fatalf("should default to false")
	}

	retry.Run(t, func(r *retry.R) {
		// This should fail due to an invalid SSL cert
		if got, want := notif.State(cid), api.HealthCritical; got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
		if !isInvalidCertificateError(notif.Output(cid)) {
			r.Fatalf("should fail with certificate error %v", notif.OutputMap())
		}
	})
}

// isInvalidCertificateError checks the error string for an untrusted certificate error.
// The specific error message is different on Linux and macOS.
//
// TODO: Revisit this when https://github.com/golang/go/issues/52010 is resolved.
// We may be able to simplify this to check only one error string.
func isInvalidCertificateError(err string) bool {
	return strings.Contains(err, "certificate signed by unknown authority") ||
		strings.Contains(err, "certificate is not trusted")
}

func mockTCPServer(network string) net.Listener {
	var (
		addr string
	)

	if network == `tcp6` {
		addr = `[::1]:0`
	} else {
		addr = `127.0.0.1:0`
	}

	listener, err := net.Listen(network, addr)
	if err != nil {
		panic(err)
	}

	return listener
}

func expectTCPStatus(t *testing.T, tcp string, status string) {
	notif := mock.NewNotify()
	logger := testutil.Logger(t)
	statusHandler := NewStatusHandler(notif, logger, 0, 0, 0)
	cid := structs.NewCheckID("foo", nil)

	check := &CheckTCP{
		CheckID:       cid,
		TCP:           tcp,
		Interval:      10 * time.Millisecond,
		Logger:        logger,
		StatusHandler: statusHandler,
	}
	check.Start()
	defer check.Stop()
	retry.Run(t, func(r *retry.R) {
		if got, want := notif.Updates(cid), 2; got < want {
			r.Fatalf("got %d updates want at least %d", got, want)
		}
		if got, want := notif.State(cid), status; got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
	})
}

func TestStatusHandlerUpdateStatusAfterConsecutiveChecksThresholdIsReached(t *testing.T) {
	t.Parallel()
	cid := structs.NewCheckID("foo", nil)
	notif := mock.NewNotify()
	logger := testutil.Logger(t)
	statusHandler := NewStatusHandler(notif, logger, 2, 2, 3)

	// Set the initial status to passing after a single success
	statusHandler.updateCheck(cid, api.HealthPassing, "bar")

	// Status should still be passing after 1 failed check only
	statusHandler.updateCheck(cid, api.HealthCritical, "bar")

	retry.Run(t, func(r *retry.R) {
		require.Equal(r, 1, notif.Updates(cid))
		require.Equal(r, api.HealthPassing, notif.State(cid))
	})

	// Status should become warning after 2 failed checks only
	statusHandler.updateCheck(cid, api.HealthCritical, "bar")

	retry.Run(t, func(r *retry.R) {
		require.Equal(r, 2, notif.Updates(cid))
		require.Equal(r, api.HealthWarning, notif.State(cid))
	})

	// Status should become critical after 4 failed checks only
	statusHandler.updateCheck(cid, api.HealthCritical, "bar")

	retry.Run(t, func(r *retry.R) {
		require.Equal(r, 3, notif.Updates(cid))
		require.Equal(r, api.HealthCritical, notif.State(cid))
	})

	// Status should be passing after 2 passing check
	statusHandler.updateCheck(cid, api.HealthPassing, "bar")

	retry.Run(t, func(r *retry.R) {
		require.Equal(r, 3, notif.Updates(cid))
		require.Equal(r, api.HealthCritical, notif.State(cid))
	})

	statusHandler.updateCheck(cid, api.HealthPassing, "bar")

	retry.Run(t, func(r *retry.R) {
		require.Equal(r, 4, notif.Updates(cid))
		require.Equal(r, api.HealthPassing, notif.State(cid))
	})
}

func TestStatusHandlerResetCountersOnNonIdenticalsConsecutiveChecks(t *testing.T) {
	t.Parallel()
	cid := structs.NewCheckID("foo", nil)
	notif := mock.NewNotify()
	logger := testutil.Logger(t)
	statusHandler := NewStatusHandler(notif, logger, 2, 2, 3)

	// Set the initial status to passing after a single success
	statusHandler.updateCheck(cid, api.HealthPassing, "bar")

	// Status should remain passing after FAIL PASS FAIL PASS FAIL sequence
	// Although we have 3 FAILS, they are not consecutive

	statusHandler.updateCheck(cid, api.HealthCritical, "bar")
	statusHandler.updateCheck(cid, api.HealthPassing, "bar")
	statusHandler.updateCheck(cid, api.HealthCritical, "bar")
	statusHandler.updateCheck(cid, api.HealthPassing, "bar")
	statusHandler.updateCheck(cid, api.HealthCritical, "bar")

	retry.Run(t, func(r *retry.R) {
		require.Equal(r, 1, notif.Updates(cid))
		require.Equal(r, api.HealthPassing, notif.State(cid))
	})

	// Warning after a 2rd consecutive FAIL
	statusHandler.updateCheck(cid, api.HealthCritical, "bar")

	retry.Run(t, func(r *retry.R) {
		require.Equal(r, 2, notif.Updates(cid))
		require.Equal(r, api.HealthWarning, notif.State(cid))
	})

	// Critical after a 3rd consecutive FAIL
	statusHandler.updateCheck(cid, api.HealthCritical, "bar")

	retry.Run(t, func(r *retry.R) {
		require.Equal(r, 3, notif.Updates(cid))
		require.Equal(r, api.HealthCritical, notif.State(cid))
	})

	// Status should remain critical after PASS FAIL PASS sequence
	statusHandler.updateCheck(cid, api.HealthPassing, "bar")
	statusHandler.updateCheck(cid, api.HealthCritical, "bar")
	statusHandler.updateCheck(cid, api.HealthPassing, "bar")

	retry.Run(t, func(r *retry.R) {
		require.Equal(r, 3, notif.Updates(cid))
		require.Equal(r, api.HealthCritical, notif.State(cid))
	})

	// Passing after a 2nd consecutive PASS
	statusHandler.updateCheck(cid, api.HealthPassing, "bar")

	retry.Run(t, func(r *retry.R) {
		require.Equal(r, 4, notif.Updates(cid))
		require.Equal(r, api.HealthPassing, notif.State(cid))
	})
}

func TestStatusHandlerWarningAndCriticalThresholdsTheSameSetsCritical(t *testing.T) {
	t.Parallel()
	cid := structs.NewCheckID("foo", nil)
	notif := mock.NewNotify()
	logger := testutil.Logger(t)
	statusHandler := NewStatusHandler(notif, logger, 2, 3, 3)

	// Set the initial status to passing after a single success
	statusHandler.updateCheck(cid, api.HealthPassing, "bar")

	// Status should remain passing after FAIL FAIL sequence
	statusHandler.updateCheck(cid, api.HealthCritical, "bar")
	statusHandler.updateCheck(cid, api.HealthCritical, "bar")

	retry.Run(t, func(r *retry.R) {
		require.Equal(r, 1, notif.Updates(cid))
		require.Equal(r, api.HealthPassing, notif.State(cid))
	})

	// Critical and not Warning after a 3rd consecutive FAIL
	statusHandler.updateCheck(cid, api.HealthCritical, "bar")

	retry.Run(t, func(r *retry.R) {
		require.Equal(r, 2, notif.Updates(cid))
		require.Equal(r, api.HealthCritical, notif.State(cid))
	})

	// Passing after consecutive PASS PASS sequence
	statusHandler.updateCheck(cid, api.HealthPassing, "bar")
	statusHandler.updateCheck(cid, api.HealthPassing, "bar")

	retry.Run(t, func(r *retry.R) {
		require.Equal(r, 3, notif.Updates(cid))
		require.Equal(r, api.HealthPassing, notif.State(cid))
	})
}

func TestStatusHandlerMaintainWarningStatusWhenCheckIsFlapping(t *testing.T) {
	t.Parallel()
	cid := structs.NewCheckID("foo", nil)
	notif := mock.NewNotify()
	logger := testutil.Logger(t)
	statusHandler := NewStatusHandler(notif, logger, 3, 3, 5)

	// Set the initial status to passing after a single success.
	statusHandler.updateCheck(cid, api.HealthPassing, "bar")

	// Status should remain passing after a FAIL FAIL sequence.
	statusHandler.updateCheck(cid, api.HealthCritical, "bar")
	statusHandler.updateCheck(cid, api.HealthCritical, "bar")

	retry.Run(t, func(r *retry.R) {
		require.Equal(r, 1, notif.Updates(cid))
		require.Equal(r, api.HealthPassing, notif.State(cid))
	})

	// Warning after a 3rd consecutive FAIL.
	statusHandler.updateCheck(cid, api.HealthCritical, "bar")

	retry.Run(t, func(r *retry.R) {
		require.Equal(r, 2, notif.Updates(cid))
		require.Equal(r, api.HealthWarning, notif.State(cid))
	})

	// Status should remain passing after PASS FAIL FAIL FAIL PASS FAIL FAIL FAIL PASS sequence.
	// Although we have 6 FAILS, they are not consecutive.
	statusHandler.updateCheck(cid, api.HealthPassing, "bar")
	statusHandler.updateCheck(cid, api.HealthCritical, "bar")
	statusHandler.updateCheck(cid, api.HealthCritical, "bar")
	statusHandler.updateCheck(cid, api.HealthCritical, "bar")

	// The status gets updated due to failuresCounter being reset
	// but the status itself remains as Warning.
	retry.Run(t, func(r *retry.R) {
		require.Equal(r, 3, notif.Updates(cid))
		require.Equal(r, api.HealthWarning, notif.State(cid))
	})

	statusHandler.updateCheck(cid, api.HealthPassing, "bar")
	statusHandler.updateCheck(cid, api.HealthCritical, "bar")
	statusHandler.updateCheck(cid, api.HealthCritical, "bar")
	statusHandler.updateCheck(cid, api.HealthCritical, "bar")

	// Status doesn'tn change, but the state update is triggered.
	retry.Run(t, func(r *retry.R) {
		require.Equal(r, 4, notif.Updates(cid))
		require.Equal(r, api.HealthWarning, notif.State(cid))
	})

	// Status should change only after 5 consecutive FAIL updates.
	statusHandler.updateCheck(cid, api.HealthPassing, "bar")
	statusHandler.updateCheck(cid, api.HealthCritical, "bar")
	statusHandler.updateCheck(cid, api.HealthCritical, "bar")
	statusHandler.updateCheck(cid, api.HealthCritical, "bar")

	// The status doesn't change, but a status update is triggered.
	retry.Run(t, func(r *retry.R) {
		require.Equal(r, 5, notif.Updates(cid))
		require.Equal(r, api.HealthWarning, notif.State(cid))
	})

	statusHandler.updateCheck(cid, api.HealthCritical, "bar")

	// The status doesn't change, but a status update is triggered.
	retry.Run(t, func(r *retry.R) {
		require.Equal(r, 6, notif.Updates(cid))
		require.Equal(r, api.HealthWarning, notif.State(cid))
	})

	statusHandler.updateCheck(cid, api.HealthCritical, "bar")

	// The FailuresBeforeCritical threshold is finally breached.
	retry.Run(t, func(r *retry.R) {
		require.Equal(r, 7, notif.Updates(cid))
		require.Equal(r, api.HealthCritical, notif.State(cid))
	})
}

func TestCheckTCPCritical(t *testing.T) {
	t.Parallel()
	var (
		tcpServer net.Listener
	)

	tcpServer = mockTCPServer(`tcp`)
	expectTCPStatus(t, `127.0.0.1:0`, api.HealthCritical)
	tcpServer.Close()
}

func TestCheckTCPPassing(t *testing.T) {
	t.Parallel()
	var (
		tcpServer net.Listener
	)

	tcpServer = mockTCPServer(`tcp`)
	expectTCPStatus(t, tcpServer.Addr().String(), api.HealthPassing)
	tcpServer.Close()

	if os.Getenv("TRAVIS") == "true" {
		t.Skip("IPV6 not supported on travis-ci")
	}
	tcpServer = mockTCPServer(`tcp6`)
	expectTCPStatus(t, tcpServer.Addr().String(), api.HealthPassing)
	tcpServer.Close()
}

func sendResponse(conn *net.UDPConn, addr *net.UDPAddr) {
	_, err := conn.WriteToUDP([]byte("healthy"), addr)
	if err != nil {
		fmt.Printf("Couldn't send response %v", err)
	}
}

func mockUDPServer(ctx context.Context, network string, port int) {

	b := make([]byte, 1024)
	addr := fmt.Sprintf(`127.0.0.1:%d`, port)

	udpAddr, err := net.ResolveUDPAddr(network, addr)
	if err != nil {
		log.Fatal("Error resolving UDP address: ", err)
	}

	ser, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatal("Error listening UDP: ", err)
	}
	defer ser.Close()

	chClose := make(chan interface{})
	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		for {
			log.Print("Waiting for UDP message")
			_, remoteaddr, err := ser.ReadFromUDP(b)
			log.Printf("Read a message from %v %s \n", remoteaddr, b)
			if err != nil {
				log.Fatalf("Error reading from UDP %s", err.Error())
			}
			sendResponse(ser, remoteaddr)
			select {
			case <-chClose:
				fmt.Println("cancelled")
				wg.Done()
				return
			default:
			}
		}
	}()

	select {
	case <-ctx.Done():
		fmt.Println("cancelled")
		close(chClose)
	}
	wg.Wait()
	return
}

func expectUDPStatus(t *testing.T, udp string, status string) {
	notif := mock.NewNotify()
	logger := testutil.Logger(t)
	statusHandler := NewStatusHandler(notif, logger, 0, 0, 0)
	cid := structs.NewCheckID("foo", nil)

	check := &CheckUDP{
		CheckID:       cid,
		UDP:           udp,
		Interval:      10 * time.Millisecond,
		Logger:        logger,
		StatusHandler: statusHandler,
	}
	check.Start()
	defer check.Stop()
	retry.Run(t, func(r *retry.R) {
		if got, want := notif.Updates(cid), 2; got < want {
			r.Fatalf("got %d updates want at least %d", got, want)
		}
		if got, want := notif.State(cid), status; got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
	})
}

func expectUDPTimeout(t *testing.T, udp string, status string) {
	notif := mock.NewNotify()
	logger := testutil.Logger(t)
	statusHandler := NewStatusHandler(notif, logger, 0, 0, 0)
	cid := structs.NewCheckID("foo", nil)

	check := &CheckUDP{
		CheckID:       cid,
		UDP:           udp,
		Interval:      10 * time.Millisecond,
		Timeout:       5 * time.Nanosecond,
		Logger:        logger,
		StatusHandler: statusHandler,
	}
	check.Start()
	defer check.Stop()
	retry.Run(t, func(r *retry.R) {
		if got, want := notif.Updates(cid), 2; got < want {
			r.Fatalf("got %d updates want at least %d", got, want)
		}
		if got, want := notif.State(cid), status; got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
	})
}

func TestCheckUDPTimeoutPassing(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	port := freeport.GetOne(t)
	serverUrl := "127.0.0.1:" + strconv.Itoa(port)

	go mockUDPServer(ctx, `udp`, port)
	expectUDPTimeout(t, serverUrl, api.HealthPassing) // Should pass since timeout is handled as success, from specification
}
func TestCheckUDPCritical(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	port := freeport.GetOne(t)
	notExistentPort := freeport.GetOne(t)
	serverUrl := "127.0.0.1:" + strconv.Itoa(notExistentPort)

	go mockUDPServer(ctx, `udp`, port)

	expectUDPStatus(t, serverUrl, api.HealthCritical) // Should be unhealthy since we never connect to mocked udp server.
}

func TestCheckUDPPassing(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	port := freeport.GetOne(t)
	serverUrl := "127.0.0.1:" + strconv.Itoa(port)

	go mockUDPServer(ctx, `udp`, port)
	expectUDPStatus(t, serverUrl, api.HealthPassing)
}

func TestCheckH2PING(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc        string
		passing     bool
		timeout     time.Duration
		connTimeout time.Duration
	}{
		{desc: "passing", passing: true, timeout: 1 * time.Second, connTimeout: 1 * time.Second},
		{desc: "failing because of time out", passing: false, timeout: 1 * time.Nanosecond, connTimeout: 1 * time.Second},
		{desc: "failing because of closed connection", passing: false, timeout: 1 * time.Nanosecond, connTimeout: 1 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { return })
			server := httptest.NewUnstartedServer(handler)
			server.EnableHTTP2 = true
			server.Config.ReadTimeout = tt.connTimeout
			server.StartTLS()
			defer server.Close()
			serverAddress := server.Listener.Addr()
			target := serverAddress.String()

			notif := mock.NewNotify()
			logger := testutil.Logger(t)
			statusHandler := NewStatusHandler(notif, logger, 0, 0, 0)
			cid := structs.NewCheckID("foo", nil)
			tlsCfg := &api.TLSConfig{
				InsecureSkipVerify: true,
			}
			tlsClientCfg, err := api.SetupTLSConfig(tlsCfg)
			if err != nil {
				t.Fatalf("%v", err)
			}
			tlsClientCfg.NextProtos = []string{http2.NextProtoTLS}

			check := &CheckH2PING{
				CheckID:         cid,
				H2PING:          target,
				Interval:        5 * time.Second,
				Timeout:         tt.timeout,
				Logger:          logger,
				TLSClientConfig: tlsClientCfg,
				StatusHandler:   statusHandler,
			}

			check.Start()
			defer check.Stop()

			if tt.passing {
				retry.Run(t, func(r *retry.R) {
					if got, want := notif.State(cid), api.HealthPassing; got != want {
						r.Fatalf("got state %q want %q", got, want)
					}
				})
			} else {
				retry.Run(t, func(r *retry.R) {
					if got, want := notif.State(cid), api.HealthCritical; got != want {
						r.Fatalf("got state %q want %q", got, want)
					}
				})
			}
		})
	}
}

func TestCheckH2PING_TLS_BadVerify(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { return })
	server := httptest.NewUnstartedServer(handler)
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()
	serverAddress := server.Listener.Addr()
	target := serverAddress.String()

	notif := mock.NewNotify()
	logger := testutil.Logger(t)
	statusHandler := NewStatusHandler(notif, logger, 0, 0, 0)
	cid := structs.NewCheckID("foo", nil)
	tlsCfg := &api.TLSConfig{}
	tlsClientCfg, err := api.SetupTLSConfig(tlsCfg)
	if err != nil {
		t.Fatalf("%v", err)
	}
	tlsClientCfg.NextProtos = []string{http2.NextProtoTLS}

	check := &CheckH2PING{
		CheckID:         cid,
		H2PING:          target,
		Interval:        5 * time.Second,
		Timeout:         2 * time.Second,
		Logger:          logger,
		TLSClientConfig: tlsClientCfg,
		StatusHandler:   statusHandler,
	}

	check.Start()
	defer check.Stop()

	insecureSkipVerifyValue := check.TLSClientConfig.InsecureSkipVerify
	if insecureSkipVerifyValue {
		t.Fatalf("The default value for InsecureSkipVerify should be false but was %v", insecureSkipVerifyValue)
	}
	retry.Run(t, func(r *retry.R) {
		if got, want := notif.State(cid), api.HealthCritical; got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
		if !isInvalidCertificateError(notif.Output(cid)) {
			r.Fatalf("should fail with certificate error %v", notif.OutputMap())
		}
	})
}
func TestCheckH2PINGInvalidListener(t *testing.T) {
	t.Parallel()

	notif := mock.NewNotify()
	logger := testutil.Logger(t)
	statusHandler := NewStatusHandler(notif, logger, 0, 0, 0)
	cid := structs.NewCheckID("foo", nil)
	tlsCfg := &api.TLSConfig{
		InsecureSkipVerify: true,
	}
	tlsClientCfg, err := api.SetupTLSConfig(tlsCfg)
	if err != nil {
		t.Fatalf("%v", err)
	}
	tlsClientCfg.NextProtos = []string{http2.NextProtoTLS}

	check := &CheckH2PING{
		CheckID:         cid,
		H2PING:          "localhost:55555",
		Interval:        5 * time.Second,
		Timeout:         1 * time.Second,
		Logger:          logger,
		TLSClientConfig: tlsClientCfg,
		StatusHandler:   statusHandler,
	}

	check.Start()
	defer check.Stop()

	retry.Run(t, func(r *retry.R) {
		if got, want := notif.State(cid), api.HealthCritical; got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
		expectedOutput := "Failed to dial to"
		if !strings.Contains(notif.Output(cid), expectedOutput) {
			r.Fatalf("should have included output %s: %v", expectedOutput, notif.OutputMap())
		}

	})
}

func TestCheckH2CPING(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc        string
		passing     bool
		timeout     time.Duration
		connTimeout time.Duration
	}{
		{desc: "passing", passing: true, timeout: 1 * time.Second, connTimeout: 1 * time.Second},
		{desc: "failing because of time out", passing: false, timeout: 1 * time.Nanosecond, connTimeout: 1 * time.Second},
		{desc: "failing because of closed connection", passing: false, timeout: 1 * time.Nanosecond, connTimeout: 1 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { return })
			h2chandler := h2c.NewHandler(handler, &http2.Server{})
			server := httptest.NewUnstartedServer(h2chandler)
			server.Config.ReadTimeout = tt.connTimeout
			server.Start()
			defer server.Close()
			serverAddress := server.Listener.Addr()
			target := serverAddress.String()

			notif := mock.NewNotify()
			logger := testutil.Logger(t)
			statusHandler := NewStatusHandler(notif, logger, 0, 0, 0)
			cid := structs.NewCheckID("foo", nil)
			check := &CheckH2PING{
				CheckID:         cid,
				H2PING:          target,
				Interval:        5 * time.Second,
				Timeout:         tt.timeout,
				Logger:          logger,
				TLSClientConfig: nil,
				StatusHandler:   statusHandler,
			}

			check.Start()
			defer check.Stop()
			if tt.passing {
				retry.Run(t, func(r *retry.R) {
					if got, want := notif.State(cid), api.HealthPassing; got != want {
						r.Fatalf("got state %q want %q", got, want)
					}
				})
			} else {
				retry.Run(t, func(r *retry.R) {
					if got, want := notif.State(cid), api.HealthCritical; got != want {
						r.Fatalf("got state %q want %q", got, want)
					}
				})
			}
		})
	}
}

func TestCheck_Docker(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	tests := []struct {
		desc     string
		handlers map[string]http.HandlerFunc
		out      *regexp.Regexp
		state    string
	}{
		{
			desc: "create exec: bad container id",
			handlers: map[string]http.HandlerFunc{
				"POST /containers/123/exec": func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(404)
				},
			},
			out:   regexp.MustCompile("^create exec failed for unknown container 123$"),
			state: api.HealthCritical,
		},
		{
			desc: "create exec: paused container",
			handlers: map[string]http.HandlerFunc{
				"POST /containers/123/exec": func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(409)
				},
			},
			out:   regexp.MustCompile("^create exec failed since container 123 is paused or stopped$"),
			state: api.HealthCritical,
		},
		{
			desc: "create exec: bad status code",
			handlers: map[string]http.HandlerFunc{
				"POST /containers/123/exec": func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(999)
					fmt.Fprint(w, "some output")
				},
			},
			out:   regexp.MustCompile("^create exec failed for container 123 with status 999: some output$"),
			state: api.HealthCritical,
		},
		{
			desc: "create exec: bad json",
			handlers: map[string]http.HandlerFunc{
				"POST /containers/123/exec": func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(201)
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, `this is not json`)
				},
			},
			out:   regexp.MustCompile("^create exec response for container 123 cannot be parsed: .*$"),
			state: api.HealthCritical,
		},
		{
			desc: "start exec: bad exec id",
			handlers: map[string]http.HandlerFunc{
				"POST /containers/123/exec": func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(201)
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, `{"Id":"456"}`)
				},
				"POST /exec/456/start": func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(404)
				},
			},
			out:   regexp.MustCompile("^start exec failed for container 123: invalid exec id 456$"),
			state: api.HealthCritical,
		},
		{
			desc: "start exec: paused container",
			handlers: map[string]http.HandlerFunc{
				"POST /containers/123/exec": func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(201)
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, `{"Id":"456"}`)
				},
				"POST /exec/456/start": func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(409)
				},
			},
			out:   regexp.MustCompile("^start exec failed since container 123 is paused or stopped$"),
			state: api.HealthCritical,
		},
		{
			desc: "start exec: bad status code",
			handlers: map[string]http.HandlerFunc{
				"POST /containers/123/exec": func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(201)
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, `{"Id":"456"}`)
				},
				"POST /exec/456/start": func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(999)
					fmt.Fprint(w, "some output")
				},
			},
			out:   regexp.MustCompile("^start exec failed for container 123 with status 999: body: some output err: <nil>$"),
			state: api.HealthCritical,
		},
		{
			desc: "inspect exec: bad exec id",
			handlers: map[string]http.HandlerFunc{
				"POST /containers/123/exec": func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(201)
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, `{"Id":"456"}`)
				},
				"POST /exec/456/start": func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(200)
					fmt.Fprint(w, "OK")
				},
				"GET /exec/456/json": func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(404)
				},
			},
			out:   regexp.MustCompile("^inspect exec failed for container 123: invalid exec id 456$"),
			state: api.HealthCritical,
		},
		{
			desc: "inspect exec: bad status code",
			handlers: map[string]http.HandlerFunc{
				"POST /containers/123/exec": func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(201)
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, `{"Id":"456"}`)
				},
				"POST /exec/456/start": func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(200)
					fmt.Fprint(w, "OK")
				},
				"GET /exec/456/json": func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(999)
					fmt.Fprint(w, "some output")
				},
			},
			out:   regexp.MustCompile("^inspect exec failed for container 123 with status 999: some output$"),
			state: api.HealthCritical,
		},
		{
			desc: "inspect exec: bad json",
			handlers: map[string]http.HandlerFunc{
				"POST /containers/123/exec": func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(201)
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, `{"Id":"456"}`)
				},
				"POST /exec/456/start": func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(200)
					fmt.Fprint(w, "OK")
				},
				"GET /exec/456/json": func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(200)
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, `this is not json`)
				},
			},
			out:   regexp.MustCompile("^inspect exec response for container 123 cannot be parsed: .*$"),
			state: api.HealthCritical,
		},
		{
			desc: "inspect exec: exit code 0: passing",
			handlers: map[string]http.HandlerFunc{
				"POST /containers/123/exec": func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(201)
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, `{"Id":"456"}`)
				},
				"POST /exec/456/start": func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(200)
					fmt.Fprint(w, "OK")
				},
				"GET /exec/456/json": func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(200)
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, `{"ExitCode":0}`)
				},
			},
			out:   regexp.MustCompile("^OK$"),
			state: api.HealthPassing,
		},
		{
			desc: "inspect exec: exit code 0: passing: truncated",
			handlers: map[string]http.HandlerFunc{
				"POST /containers/123/exec": func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(201)
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, `{"Id":"456"}`)
				},
				"POST /exec/456/start": func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(200)
					fmt.Fprint(w, "01234567890123456789OK") // more than 20 bytes
				},
				"GET /exec/456/json": func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(200)
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, `{"ExitCode":0}`)
				},
			},
			out:   regexp.MustCompile("^Captured 20 of 22 bytes\n...\n234567890123456789OK$"),
			state: api.HealthPassing,
		},
		{
			desc: "inspect exec: exit code 1: warning",
			handlers: map[string]http.HandlerFunc{
				"POST /containers/123/exec": func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(201)
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, `{"Id":"456"}`)
				},
				"POST /exec/456/start": func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(200)
					fmt.Fprint(w, "WARN")
				},
				"GET /exec/456/json": func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(200)
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, `{"ExitCode":1}`)
				},
			},
			out:   regexp.MustCompile("^WARN$"),
			state: api.HealthWarning,
		},
		{
			desc: "inspect exec: exit code 2: critical",
			handlers: map[string]http.HandlerFunc{
				"POST /containers/123/exec": func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(201)
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, `{"Id":"456"}`)
				},
				"POST /exec/456/start": func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(200)
					fmt.Fprint(w, "NOK")
				},
				"GET /exec/456/json": func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(200)
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, `{"ExitCode":2}`)
				},
			},
			out:   regexp.MustCompile("^NOK$"),
			state: api.HealthCritical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				x := r.Method + " " + r.RequestURI
				h := tt.handlers[x]
				if h == nil {
					t.Fatalf("bad url %s", x)
				}
				h(w, r)
			}))
			defer srv.Close()

			// create a docker client with a tiny output buffer
			// to test the truncation
			c, err := NewDockerClient(srv.URL, 20)
			if err != nil {
				t.Fatal(err)
			}

			notif, upd := mock.NewNotifyChan()
			logger := testutil.Logger(t)
			statusHandler := NewStatusHandler(notif, logger, 0, 0, 0)
			id := structs.NewCheckID("chk", nil)

			check := &CheckDocker{
				CheckID:           id,
				ScriptArgs:        []string{"/health.sh"},
				DockerContainerID: "123",
				Interval:          25 * time.Millisecond,
				Client:            c,
				StatusHandler:     statusHandler,
			}
			check.Start()
			defer check.Stop()

			<-upd // wait for update

			if got, want := notif.Output(id), tt.out; !want.MatchString(got) {
				t.Fatalf("got %q want %q", got, want)
			}
			if got, want := notif.State(id), tt.state; got != want {
				t.Fatalf("got status %q want %q", got, want)
			}
		})
	}
}
