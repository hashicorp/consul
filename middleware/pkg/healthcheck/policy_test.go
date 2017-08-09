package healthcheck

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

var workableServer *httptest.Server

func TestMain(m *testing.M) {
	workableServer = httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// do nothing
		}))
	r := m.Run()
	workableServer.Close()
	os.Exit(r)
}

type customPolicy struct{}

func (r *customPolicy) Select(pool HostPool) *UpstreamHost {
	return pool[0]
}

func testPool() HostPool {
	pool := []*UpstreamHost{
		{
			Name: workableServer.URL, // this should resolve (healthcheck test)
		},
		{
			Name: "http://shouldnot.resolve", // this shouldn't
		},
		{
			Name: "http://C",
		},
	}
	return HostPool(pool)
}

func TestRegisterPolicy(t *testing.T) {
	name := "custom"
	customPolicy := &customPolicy{}
	RegisterPolicy(name, func() Policy { return customPolicy })
	if _, ok := SupportedPolicies[name]; !ok {
		t.Error("Expected supportedPolicies to have a custom policy.")
	}

}

func TestHealthCheck(t *testing.T) {
	log.SetOutput(ioutil.Discard)

	u := &HealthCheck{
		Hosts:       testPool(),
		FailTimeout: 10 * time.Second,
		Future:      60 * time.Second,
		MaxFails:    1,
	}

	u.healthCheck()
	// sleep a bit, it's async now
	time.Sleep(time.Duration(2 * time.Second))

	if u.Hosts[0].Down() {
		t.Error("Expected first host in testpool to not fail healthcheck.")
	}
	if !u.Hosts[1].Down() {
		t.Error("Expected second host in testpool to fail healthcheck.")
	}
}

func TestSelect(t *testing.T) {
	u := &HealthCheck{
		Hosts:       testPool()[:3],
		FailTimeout: 10 * time.Second,
		Future:      60 * time.Second,
		MaxFails:    1,
	}
	u.Hosts[0].OkUntil = time.Unix(0, 0)
	u.Hosts[1].OkUntil = time.Unix(0, 0)
	u.Hosts[2].OkUntil = time.Unix(0, 0)
	if h := u.Select(); h != nil {
		t.Error("Expected select to return nil as all host are down")
	}
	u.Hosts[2].OkUntil = time.Time{}
	if h := u.Select(); h == nil {
		t.Error("Expected select to not return nil")
	}
}

func TestRoundRobinPolicy(t *testing.T) {
	pool := testPool()
	rrPolicy := &RoundRobin{}
	h := rrPolicy.Select(pool)
	// First selected host is 1, because counter starts at 0
	// and increments before host is selected
	if h != pool[1] {
		t.Error("Expected first round robin host to be second host in the pool.")
	}
	h = rrPolicy.Select(pool)
	if h != pool[2] {
		t.Error("Expected second round robin host to be third host in the pool.")
	}
	// mark host as down
	pool[0].OkUntil = time.Unix(0, 0)
	h = rrPolicy.Select(pool)
	if h != pool[1] {
		t.Error("Expected third round robin host to be first host in the pool.")
	}
}

func TestLeastConnPolicy(t *testing.T) {
	pool := testPool()
	lcPolicy := &LeastConn{}
	pool[0].Conns = 10
	pool[1].Conns = 10
	h := lcPolicy.Select(pool)
	if h != pool[2] {
		t.Error("Expected least connection host to be third host.")
	}
	pool[2].Conns = 100
	h = lcPolicy.Select(pool)
	if h != pool[0] && h != pool[1] {
		t.Error("Expected least connection host to be first or second host.")
	}
}

func TestCustomPolicy(t *testing.T) {
	pool := testPool()
	customPolicy := &customPolicy{}
	h := customPolicy.Select(pool)
	if h != pool[0] {
		t.Error("Expected custom policy host to be the first host.")
	}
}
