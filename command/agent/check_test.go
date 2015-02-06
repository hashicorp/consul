package agent

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/testutil"
)

type MockNotify struct {
	state   map[string]string
	updates map[string]int
	output  map[string]string
}

func (m *MockNotify) UpdateCheck(id, status, output string) {
	m.state[id] = status
	old := m.updates[id]
	m.updates[id] = old + 1
	m.output[id] = output
}

func expectStatus(t *testing.T, script, status string) {
	mock := &MockNotify{
		state:   make(map[string]string),
		updates: make(map[string]int),
		output:  make(map[string]string),
	}
	check := &CheckMonitor{
		Notify:   mock,
		CheckID:  "foo",
		Script:   script,
		Interval: 10 * time.Millisecond,
		Logger:   log.New(os.Stderr, "", log.LstdFlags),
	}
	check.Start()
	defer check.Stop()

	testutil.WaitForResult(func() (bool, error) {
		// Should have at least 2 updates
		if mock.updates["foo"] < 2 {
			return false, fmt.Errorf("should have 2 updates %v", mock.updates)
		}

		if mock.state["foo"] != status {
			return false, fmt.Errorf("should be %v %v", status, mock.state)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})
}

func TestCheckMonitor_Passing(t *testing.T) {
	expectStatus(t, "exit 0", structs.HealthPassing)
}

func TestCheckMonitor_Warning(t *testing.T) {
	expectStatus(t, "exit 1", structs.HealthWarning)
}

func TestCheckMonitor_Critical(t *testing.T) {
	expectStatus(t, "exit 2", structs.HealthCritical)
}

func TestCheckMonitor_BadCmd(t *testing.T) {
	expectStatus(t, "foobarbaz", structs.HealthCritical)
}

func TestCheckMonitor_RandomStagger(t *testing.T) {
	mock := &MockNotify{
		state:   make(map[string]string),
		updates: make(map[string]int),
		output:  make(map[string]string),
	}
	check := &CheckMonitor{
		Notify:   mock,
		CheckID:  "foo",
		Script:   "exit 0",
		Interval: 25 * time.Millisecond,
		Logger:   log.New(os.Stderr, "", log.LstdFlags),
	}
	check.Start()
	defer check.Stop()

	time.Sleep(50 * time.Millisecond)

	// Should have at least 1 update
	if mock.updates["foo"] < 1 {
		t.Fatalf("should have 1 or more updates %v", mock.updates)
	}

	if mock.state["foo"] != structs.HealthPassing {
		t.Fatalf("should be %v %v", structs.HealthPassing, mock.state)
	}
}

func TestCheckMonitor_LimitOutput(t *testing.T) {
	mock := &MockNotify{
		state:   make(map[string]string),
		updates: make(map[string]int),
		output:  make(map[string]string),
	}
	check := &CheckMonitor{
		Notify:   mock,
		CheckID:  "foo",
		Script:   "od -N 81920 /dev/urandom",
		Interval: 25 * time.Millisecond,
		Logger:   log.New(os.Stderr, "", log.LstdFlags),
	}
	check.Start()
	defer check.Stop()

	time.Sleep(50 * time.Millisecond)

	// Allow for extra bytes for the truncation message
	if len(mock.output["foo"]) > CheckBufSize+100 {
		t.Fatalf("output size is too long")
	}
}

func TestCheckTTL(t *testing.T) {
	mock := &MockNotify{
		state:   make(map[string]string),
		updates: make(map[string]int),
		output:  make(map[string]string),
	}
	check := &CheckTTL{
		Notify:  mock,
		CheckID: "foo",
		TTL:     100 * time.Millisecond,
		Logger:  log.New(os.Stderr, "", log.LstdFlags),
	}
	check.Start()
	defer check.Stop()

	time.Sleep(50 * time.Millisecond)
	check.SetStatus(structs.HealthPassing, "")

	if mock.updates["foo"] != 1 {
		t.Fatalf("should have 1 updates %v", mock.updates)
	}

	if mock.state["foo"] != structs.HealthPassing {
		t.Fatalf("should be passing %v", mock.state)
	}

	// Ensure we don't fail early
	time.Sleep(75 * time.Millisecond)
	if mock.updates["foo"] != 1 {
		t.Fatalf("should have 1 updates %v", mock.updates)
	}

	// Wait for the TTL to expire
	time.Sleep(75 * time.Millisecond)

	if mock.updates["foo"] != 2 {
		t.Fatalf("should have 2 updates %v", mock.updates)
	}

	if mock.state["foo"] != structs.HealthCritical {
		t.Fatalf("should be critical %v", mock.state)
	}
}

func mockHTTPServer(responseCode int) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(responseCode)
		return
	})

	return httptest.NewServer(mux)
}

func expectHTTPStatus(t *testing.T, url string, status string) {
	mock := &MockNotify{
		state:   make(map[string]string),
		updates: make(map[string]int),
		output:  make(map[string]string),
	}
	check := &CheckHTTP{
		Notify:   mock,
		CheckID:  "foo",
		HTTP:     url,
		Interval: 10 * time.Millisecond,
		Logger:   log.New(os.Stderr, "", log.LstdFlags),
	}
	check.Start()
	defer check.Stop()

	time.Sleep(50 * time.Millisecond)

	// Should have at least 2 updates
	if mock.updates["foo"] < 2 {
		t.Fatalf("should have 2 updates %v", mock.updates)
	}

	if mock.state["foo"] != status {
		t.Fatalf("should be %v %v", status, mock.state)
	}
}

func TestCheckHTTPCritical(t *testing.T) {
	// var server *httptest.Server

	server := mockHTTPServer(150)
	fmt.Println(server.URL)
	expectHTTPStatus(t, server.URL, "critical")
	server.Close()

	// 2xx - 1
	server = mockHTTPServer(199)
	expectHTTPStatus(t, server.URL, "critical")
	server.Close()

	// 2xx + 1
	server = mockHTTPServer(300)
	expectHTTPStatus(t, server.URL, "critical")
	server.Close()

	server = mockHTTPServer(400)
	expectHTTPStatus(t, server.URL, "critical")
	server.Close()

	server = mockHTTPServer(500)
	expectHTTPStatus(t, server.URL, "critical")
	server.Close()
}

func TestCheckHTTPPassing(t *testing.T) {
	var server *httptest.Server

	server = mockHTTPServer(200)
	expectHTTPStatus(t, server.URL, "passing")
	server.Close()

	server = mockHTTPServer(201)
	expectHTTPStatus(t, server.URL, "passing")
	server.Close()

	server = mockHTTPServer(250)
	expectHTTPStatus(t, server.URL, "passing")
	server.Close()

	server = mockHTTPServer(299)
	expectHTTPStatus(t, server.URL, "passing")
	server.Close()
}

func TestCheckHTTPWarning(t *testing.T) {
	server := mockHTTPServer(429)
	expectHTTPStatus(t, server.URL, "warning")
	server.Close()
}

func mockSlowHTTPServer(responseCode int, sleep time.Duration) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(sleep)
		w.WriteHeader(responseCode)
		return
	})

	return httptest.NewServer(mux)
}

func TestCheckHTTPTimeout(t *testing.T) {
	server := mockSlowHTTPServer(200, 10*time.Millisecond)
	defer server.Close()

	mock := &MockNotify{
		state:   make(map[string]string),
		updates: make(map[string]int),
		output:  make(map[string]string),
	}

	check := &CheckHTTP{
		Notify:   mock,
		CheckID:  "bar",
		HTTP:     server.URL,
		Timeout:  5 * time.Millisecond,
		Interval: 10 * time.Millisecond,
		Logger:   log.New(os.Stderr, "", log.LstdFlags),
	}

	check.Start()
	defer check.Stop()

	time.Sleep(50 * time.Millisecond)

	// Should have at least 2 updates
	if mock.updates["bar"] < 2 {
		t.Fatalf("should have at least 2 updates %v", mock.updates)
	}

	if mock.state["bar"] != "critical" {
		t.Fatalf("should be critical %v", mock.state)
	}
}
