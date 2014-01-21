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
}

func (m *MockNotify) UpdateCheck(id, status string) {
	m.state[id] = status
	old := m.updates[id]
	m.updates[id] = old + 1
}

func expectStatus(t *testing.T, script, status string) {
	mock := &MockNotify{
		state:   make(map[string]string),
		updates: make(map[string]int),
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
