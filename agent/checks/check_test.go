package checks

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/mock"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/consul/types"
	uuid "github.com/hashicorp/go-uuid"
)

func uniqueID() string {
	id, err := uuid.GenerateUUID()
	if err != nil {
		panic(err)
	}
	return id
}

func TestCheckMonitor_Script(t *testing.T) {
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
			check := &CheckMonitor{
				Notify:   notif,
				CheckID:  types.CheckID("foo"),
				Script:   tt.script,
				Interval: 25 * time.Millisecond,
				Logger:   log.New(ioutil.Discard, uniqueID(), log.LstdFlags),
			}
			check.Start()
			defer check.Stop()
			retry.Run(t, func(r *retry.R) {
				if got, want := notif.Updates("foo"), 2; got < want {
					r.Fatalf("got %d updates want at least %d", got, want)
				}
				if got, want := notif.State("foo"), tt.status; got != want {
					r.Fatalf("got state %q want %q", got, want)
				}
			})
		})
	}
}

func TestCheckMonitor_Args(t *testing.T) {
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
			check := &CheckMonitor{
				Notify:     notif,
				CheckID:    types.CheckID("foo"),
				ScriptArgs: tt.args,
				Interval:   25 * time.Millisecond,
				Logger:     log.New(ioutil.Discard, uniqueID(), log.LstdFlags),
			}
			check.Start()
			defer check.Stop()
			retry.Run(t, func(r *retry.R) {
				if got, want := notif.Updates("foo"), 2; got < want {
					r.Fatalf("got %d updates want at least %d", got, want)
				}
				if got, want := notif.State("foo"), tt.status; got != want {
					r.Fatalf("got state %q want %q", got, want)
				}
			})
		})
	}
}

func TestCheckMonitor_Timeout(t *testing.T) {
	// t.Parallel() // timing test. no parallel
	notif := mock.NewNotify()
	check := &CheckMonitor{
		Notify:     notif,
		CheckID:    types.CheckID("foo"),
		ScriptArgs: []string{"sh", "-c", "sleep 1 && exit 0"},
		Interval:   50 * time.Millisecond,
		Timeout:    25 * time.Millisecond,
		Logger:     log.New(ioutil.Discard, uniqueID(), log.LstdFlags),
	}
	check.Start()
	defer check.Stop()

	time.Sleep(250 * time.Millisecond)

	// Should have at least 2 updates
	if notif.Updates("foo") < 2 {
		t.Fatalf("should have at least 2 updates %v", notif.UpdatesMap())
	}
	if notif.State("foo") != "critical" {
		t.Fatalf("should be critical %v", notif.StateMap())
	}
}

func TestCheckMonitor_RandomStagger(t *testing.T) {
	// t.Parallel() // timing test. no parallel
	notif := mock.NewNotify()
	check := &CheckMonitor{
		Notify:     notif,
		CheckID:    types.CheckID("foo"),
		ScriptArgs: []string{"sh", "-c", "exit 0"},
		Interval:   25 * time.Millisecond,
		Logger:     log.New(ioutil.Discard, uniqueID(), log.LstdFlags),
	}
	check.Start()
	defer check.Stop()

	time.Sleep(500 * time.Millisecond)

	// Should have at least 1 update
	if notif.Updates("foo") < 1 {
		t.Fatalf("should have 1 or more updates %v", notif.UpdatesMap())
	}

	if notif.State("foo") != api.HealthPassing {
		t.Fatalf("should be %v %v", api.HealthPassing, notif.StateMap())
	}
}

func TestCheckMonitor_LimitOutput(t *testing.T) {
	t.Parallel()
	notif := mock.NewNotify()
	check := &CheckMonitor{
		Notify:     notif,
		CheckID:    types.CheckID("foo"),
		ScriptArgs: []string{"od", "-N", "81920", "/dev/urandom"},
		Interval:   25 * time.Millisecond,
		Logger:     log.New(ioutil.Discard, uniqueID(), log.LstdFlags),
	}
	check.Start()
	defer check.Stop()

	time.Sleep(50 * time.Millisecond)

	// Allow for extra bytes for the truncation message
	if len(notif.Output("foo")) > BufSize+100 {
		t.Fatalf("output size is too long")
	}
}

func TestCheckTTL(t *testing.T) {
	// t.Parallel() // timing test. no parallel
	notif := mock.NewNotify()
	check := &CheckTTL{
		Notify:  notif,
		CheckID: types.CheckID("foo"),
		TTL:     200 * time.Millisecond,
		Logger:  log.New(ioutil.Discard, uniqueID(), log.LstdFlags),
	}
	check.Start()
	defer check.Stop()

	time.Sleep(100 * time.Millisecond)
	check.SetStatus(api.HealthPassing, "test-output")

	if notif.Updates("foo") != 1 {
		t.Fatalf("should have 1 updates %v", notif.UpdatesMap())
	}

	if notif.State("foo") != api.HealthPassing {
		t.Fatalf("should be passing %v", notif.StateMap())
	}

	// Ensure we don't fail early
	time.Sleep(150 * time.Millisecond)
	if notif.Updates("foo") != 1 {
		t.Fatalf("should have 1 updates %v", notif.UpdatesMap())
	}

	// Wait for the TTL to expire
	time.Sleep(150 * time.Millisecond)

	if notif.Updates("foo") != 2 {
		t.Fatalf("should have 2 updates %v", notif.UpdatesMap())
	}

	if notif.State("foo") != api.HealthCritical {
		t.Fatalf("should be critical %v", notif.StateMap())
	}

	if !strings.Contains(notif.Output("foo"), "test-output") {
		t.Fatalf("should have retained output %v", notif.OutputMap())
	}
}

func TestCheckHTTP(t *testing.T) {
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
		{code: 150, status: api.HealthCritical},
		{code: 199, status: api.HealthCritical},
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
				body := bytes.Repeat([]byte{'a'}, 2*BufSize)
				w.WriteHeader(tt.code)
				w.Write(body)
			}))
			defer server.Close()

			notif := mock.NewNotify()
			check := &CheckHTTP{
				Notify:   notif,
				CheckID:  types.CheckID("foo"),
				HTTP:     server.URL,
				Method:   tt.method,
				Header:   tt.header,
				Interval: 10 * time.Millisecond,
				Logger:   log.New(ioutil.Discard, uniqueID(), log.LstdFlags),
			}
			check.Start()
			defer check.Stop()

			retry.Run(t, func(r *retry.R) {
				if got, want := notif.Updates("foo"), 2; got < want {
					r.Fatalf("got %d updates want at least %d", got, want)
				}
				if got, want := notif.State("foo"), tt.status; got != want {
					r.Fatalf("got state %q want %q", got, want)
				}
				// Allow slightly more data than BufSize, for the header
				if n := len(notif.Output("foo")); n > (BufSize + 256) {
					r.Fatalf("output too long: %d (%d-byte limit)", n, BufSize)
				}
			})
		})
	}
}

func TestCheckHTTPTimeout(t *testing.T) {
	t.Parallel()
	timeout := 5 * time.Millisecond
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		time.Sleep(2 * timeout)
	}))
	defer server.Close()

	notif := mock.NewNotify()
	check := &CheckHTTP{
		Notify:   notif,
		CheckID:  types.CheckID("bar"),
		HTTP:     server.URL,
		Timeout:  timeout,
		Interval: 10 * time.Millisecond,
		Logger:   log.New(ioutil.Discard, uniqueID(), log.LstdFlags),
	}

	check.Start()
	defer check.Stop()
	retry.Run(t, func(r *retry.R) {
		if got, want := notif.Updates("bar"), 2; got < want {
			r.Fatalf("got %d updates want at least %d", got, want)
		}
		if got, want := notif.State("bar"), api.HealthCritical; got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
	})
}

func TestCheckHTTP_disablesKeepAlives(t *testing.T) {
	t.Parallel()
	check := &CheckHTTP{
		CheckID:  types.CheckID("foo"),
		HTTP:     "http://foo.bar/baz",
		Interval: 10 * time.Second,
		Logger:   log.New(ioutil.Discard, uniqueID(), log.LstdFlags),
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
		body := bytes.Repeat([]byte{'a'}, 2*BufSize)
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
	check := &CheckHTTP{
		Notify:          notif,
		CheckID:         types.CheckID("skipverify_true"),
		HTTP:            server.URL,
		Interval:        25 * time.Millisecond,
		Logger:          log.New(ioutil.Discard, uniqueID(), log.LstdFlags),
		TLSClientConfig: tlsClientConfig,
	}

	check.Start()
	defer check.Stop()

	if !check.httpClient.Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify {
		t.Fatalf("should be true")
	}

	retry.Run(t, func(r *retry.R) {
		if got, want := notif.State("skipverify_true"), api.HealthPassing; got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
	})
}

func TestCheckHTTP_TLS_BadVerify(t *testing.T) {
	t.Parallel()
	server := httptest.NewTLSServer(largeBodyHandler(200))
	defer server.Close()

	tlsClientConfig, err := api.SetupTLSConfig(&api.TLSConfig{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	notif := mock.NewNotify()
	check := &CheckHTTP{
		Notify:          notif,
		CheckID:         types.CheckID("skipverify_false"),
		HTTP:            server.URL,
		Interval:        100 * time.Millisecond,
		Logger:          log.New(ioutil.Discard, uniqueID(), log.LstdFlags),
		TLSClientConfig: tlsClientConfig,
	}

	check.Start()
	defer check.Stop()

	if check.httpClient.Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify {
		t.Fatalf("should default to false")
	}

	retry.Run(t, func(r *retry.R) {
		// This should fail due to an invalid SSL cert
		if got, want := notif.State("skipverify_false"), api.HealthCritical; got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
		if !strings.Contains(notif.Output("skipverify_false"), "certificate signed by unknown authority") {
			r.Fatalf("should fail with certificate error %v", notif.OutputMap())
		}
	})
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
	check := &CheckTCP{
		Notify:   notif,
		CheckID:  types.CheckID("foo"),
		TCP:      tcp,
		Interval: 10 * time.Millisecond,
		Logger:   log.New(ioutil.Discard, uniqueID(), log.LstdFlags),
	}
	check.Start()
	defer check.Stop()
	retry.Run(t, func(r *retry.R) {
		if got, want := notif.Updates("foo"), 2; got < want {
			r.Fatalf("got %d updates want at least %d", got, want)
		}
		if got, want := notif.State("foo"), status; got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
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

func TestCheck_Docker(t *testing.T) {
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
			id := types.CheckID("chk")
			check := &CheckDocker{
				Notify:            notif,
				CheckID:           id,
				ScriptArgs:        []string{"/health.sh"},
				DockerContainerID: "123",
				Interval:          25 * time.Millisecond,
				Client:            c,
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
