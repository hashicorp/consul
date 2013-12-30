package agent

import (
	"fmt"
	"github.com/hashicorp/serf/serf"
	"github.com/hashicorp/serf/testutil"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

type rpcParts struct {
	dir    string
	client *RPCClient
	agent  *Agent
	rpc    *AgentRPC
}

func (r *rpcParts) Close() {
	r.client.Close()
	r.rpc.Shutdown()
	r.agent.Shutdown()
	os.RemoveAll(r.dir)
}

// testRPCClient returns an RPCClient connected to an RPC server that
// serves only this connection.
func testRPCClient(t *testing.T) *rpcParts {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	lw := NewLogWriter(512)
	mult := io.MultiWriter(os.Stderr, lw)

	conf := nextConfig()
	dir, agent := makeAgentLog(t, conf, mult)
	rpc := NewAgentRPC(agent, l, mult, lw)

	rpcClient, err := NewRPCClient(l.Addr().String())
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	return &rpcParts{
		dir:    dir,
		client: rpcClient,
		agent:  agent,
		rpc:    rpc,
	}
}

func TestRPCClientForceLeave(t *testing.T) {
	p1 := testRPCClient(t)
	p2 := testRPCClient(t)
	defer p1.Close()
	defer p2.Close()
	testutil.Yield()

	s2Addr := fmt.Sprintf("127.0.0.1:%d", p2.agent.config.SerfLanPort)
	if _, err := p1.agent.JoinLAN([]string{s2Addr}); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	if err := p2.agent.Shutdown(); err != nil {
		t.Fatalf("err: %s", err)
	}

	time.Sleep(time.Second)

	if err := p1.client.ForceLeave(p2.agent.config.NodeName); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	m := p1.agent.LANMembers()
	if len(m) != 2 {
		t.Fatalf("should have 2 members: %#v", m)
	}

	if m[1].Status != serf.StatusLeft {
		t.Fatalf("should be left: %#v %v", m[1], m[1].Status == serf.StatusLeft)
	}
}

func TestRPCClientJoinLAN(t *testing.T) {
	p1 := testRPCClient(t)
	p2 := testRPCClient(t)
	defer p1.Close()
	defer p2.Close()
	testutil.Yield()

	s2Addr := fmt.Sprintf("127.0.0.1:%d", p2.agent.config.SerfLanPort)
	n, err := p1.client.Join([]string{s2Addr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if n != 1 {
		t.Fatalf("n != 1: %d", n)
	}
}

func TestRPCClientJoinWAN(t *testing.T) {
	p1 := testRPCClient(t)
	p2 := testRPCClient(t)
	defer p1.Close()
	defer p2.Close()
	testutil.Yield()

	s2Addr := fmt.Sprintf("127.0.0.1:%d", p2.agent.config.SerfWanPort)
	n, err := p1.client.Join([]string{s2Addr}, true)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if n != 1 {
		t.Fatalf("n != 1: %d", n)
	}
}

func TestRPCClientLANMembers(t *testing.T) {
	p1 := testRPCClient(t)
	p2 := testRPCClient(t)
	defer p1.Close()
	defer p2.Close()
	testutil.Yield()

	mem, err := p1.client.LANMembers()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if len(mem) != 1 {
		t.Fatalf("bad: %#v", mem)
	}

	s2Addr := fmt.Sprintf("127.0.0.1:%d", p2.agent.config.SerfLanPort)
	_, err = p1.client.Join([]string{s2Addr}, false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	mem, err = p1.client.LANMembers()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if len(mem) != 2 {
		t.Fatalf("bad: %#v", mem)
	}
}

func TestRPCClientWANMembers(t *testing.T) {
	p1 := testRPCClient(t)
	p2 := testRPCClient(t)
	defer p1.Close()
	defer p2.Close()
	testutil.Yield()

	mem, err := p1.client.WANMembers()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if len(mem) != 1 {
		t.Fatalf("bad: %#v", mem)
	}

	s2Addr := fmt.Sprintf("127.0.0.1:%d", p2.agent.config.SerfWanPort)
	_, err = p1.client.Join([]string{s2Addr}, true)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	mem, err = p1.client.WANMembers()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if len(mem) != 2 {
		t.Fatalf("bad: %#v", mem)
	}
}

func TestRPCClientLeave(t *testing.T) {
	p1 := testRPCClient(t)
	defer p1.Close()
	testutil.Yield()

	if err := p1.client.Leave(); err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.Yield()

	select {
	case <-p1.agent.ShutdownCh():
	default:
		t.Fatalf("agent should be shutdown!")
	}
}

func TestRPCClientMonitor(t *testing.T) {
	p1 := testRPCClient(t)
	defer p1.Close()
	testutil.Yield()

	eventCh := make(chan string, 64)
	if handle, err := p1.client.Monitor("debug", eventCh); err != nil {
		t.Fatalf("err: %s", err)
	} else {
		defer p1.client.Stop(handle)
	}

	testutil.Yield()

	found := false
OUTER1:
	for {
		select {
		case e := <-eventCh:
			if strings.Contains(e, "Accepted client") {
				found = true
			}
		default:
			break OUTER1
		}
	}
	if !found {
		t.Fatalf("should log client accept")
	}

	// Join a bad thing to generate more events
	p1.agent.JoinLAN(nil)
	testutil.Yield()

	found = false
OUTER2:
	for {
		select {
		case e := <-eventCh:
			if strings.Contains(e, "joining") {
				found = true
			}
		default:
			break OUTER2
		}
	}
	if !found {
		t.Fatalf("should log joining")
	}
}
