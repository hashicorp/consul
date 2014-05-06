package agent

import (
	"github.com/hashicorp/consul/consul/structs"
	"log"
	"os"
	"testing"
	"time"
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
		Interval: 25 * time.Millisecond,
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

func TestCheckMonitor_LimitOutput(t *testing.T) {
	mock := &MockNotify{
		state:   make(map[string]string),
		updates: make(map[string]int),
		output:  make(map[string]string),
	}
	check := &CheckMonitor{
		Notify:   mock,
		CheckID:  "foo",
		Script:   "dd if=/dev/urandom bs=8192 count=10",
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
