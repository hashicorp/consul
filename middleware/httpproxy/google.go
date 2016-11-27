package httpproxy

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/miekg/coredns/middleware/proxy"

	"github.com/miekg/dns"
)

// immediate retries until this duration ends or we get a nil host.
var tryDuration = 60 * time.Second

type google struct {
	client   *http.Client
	upstream *simpleUpstream
	addr     *simpleUpstream
	quit     chan bool
	sync.RWMutex
}

func newGoogle() *google { return &google{client: newClient(ghost), quit: make(chan bool)} }

func (g *google) Exchange(req *dns.Msg) (*dns.Msg, error) {
	v := url.Values{}

	v.Set("name", req.Question[0].Name)
	v.Set("type", fmt.Sprintf("%d", req.Question[0].Qtype))

	start := time.Now()

	for time.Now().Sub(start) < tryDuration {

		g.RLock()
		addr := g.addr.Select()
		g.RUnlock()

		if addr == nil {
			return nil, fmt.Errorf("no healthy upstream http hosts")
		}

		atomic.AddInt64(&addr.Conns, 1)

		buf, backendErr := g.do(addr.Name, v.Encode())

		atomic.AddInt64(&addr.Conns, -1)

		if backendErr == nil {
			gm := new(googleMsg)
			if err := json.Unmarshal(buf, gm); err != nil {
				return nil, err
			}

			m, err := toMsg(gm)
			if err != nil {
				return nil, err
			}

			m.Id = req.Id
			return m, nil
		}

		log.Printf("[WARNING] Failed to connect to HTTPS backend %q: %s", ghost, backendErr)

		timeout := addr.FailTimeout
		if timeout == 0 {
			timeout = 5 * time.Second
		}
		atomic.AddInt32(&addr.Fails, 1)
		go func(host *proxy.UpstreamHost, timeout time.Duration) {
			time.Sleep(timeout)
			atomic.AddInt32(&host.Fails, -1)
		}(addr, timeout)
	}

	return nil, errUnreachable
}

// OnStartup looks up the IP address for "ghost" every 30 seconds.
func (g *google) OnStartup() error {
	r := new(dns.Msg)
	r.SetQuestion(dns.Fqdn(ghost), dns.TypeA)
	new, err := g.lookup(r)
	if err != nil {
		return err
	}
	r.SetQuestion(dns.Fqdn(ghost), dns.TypeAAAA)
	new6, err := g.lookup(r)
	if err != nil {
		return err
	}

	up, _ := newSimpleUpstream(append(new, new6...))
	g.Lock()
	g.addr = up
	g.Unlock()

	go func() {
		tick := time.NewTicker(30 * time.Second)

		for {
			select {
			case <-tick.C:

				r.SetQuestion(dns.Fqdn(ghost), dns.TypeA)
				new, err := g.lookup(r)
				if err != nil {
					log.Printf("[WARNING] Failed to lookup A records %q: %s", ghost, err)
					continue
				}
				r.SetQuestion(dns.Fqdn(ghost), dns.TypeAAAA)
				new6, err := g.lookup(r)
				if err != nil {
					log.Printf("[WARNING] Failed to lookup AAAA records %q: %s", ghost, err)
					continue
				}

				up, _ := newSimpleUpstream(append(new, new6...))
				g.Lock()
				g.addr = up
				g.Unlock()
			case <-g.quit:
				return
			}
		}
	}()

	return nil
}

func (g *google) OnShutdown() error {
	g.quit <- true
	return nil
}

func (g *google) SetUpstream(u *simpleUpstream) error {
	g.upstream = u
	return nil
}

func (g *google) lookup(r *dns.Msg) ([]string, error) {
	c := new(dns.Client)
	start := time.Now()

	for time.Now().Sub(start) < tryDuration {
		host := g.upstream.Select()
		if host == nil {
			return nil, fmt.Errorf("no healthy upstream hosts")
		}

		atomic.AddInt64(&host.Conns, 1)

		m, _, backendErr := c.Exchange(r, host.Name)

		atomic.AddInt64(&host.Conns, -1)

		if backendErr == nil {
			if len(m.Answer) == 0 {
				return nil, fmt.Errorf("no answer section in response")
			}
			ret := []string{}
			for _, an := range m.Answer {
				if a, ok := an.(*dns.A); ok {
					ret = append(ret, net.JoinHostPort(a.A.String(), "443"))
				}
				if a, ok := an.(*dns.AAAA); ok {
					ret = append(ret, net.JoinHostPort(a.AAAA.String(), "443"))
				}
			}
			if len(ret) > 0 {
				return ret, nil
			}

			return nil, fmt.Errorf("no address records in answer section")
		}

		timeout := host.FailTimeout
		if timeout == 0 {
			timeout = 7 * time.Second
		}
		atomic.AddInt32(&host.Fails, 1)
		go func(host *proxy.UpstreamHost, timeout time.Duration) {
			time.Sleep(timeout)
			atomic.AddInt32(&host.Fails, -1)
		}(host, timeout)
	}
	return nil, fmt.Errorf("no healthy upstream hosts")
}

func (g *google) do(addr, json string) ([]byte, error) {
	url := "https://" + addr + "/resolve?" + json
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Host = ghost

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}

	buf, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get 200 status code, got %d", resp.StatusCode)
	}

	return buf, nil
}

func toMsg(g *googleMsg) (*dns.Msg, error) {
	m := new(dns.Msg)
	m.Rcode = g.Status
	m.Truncated = g.TC
	m.RecursionDesired = g.RD
	m.RecursionAvailable = g.RA
	m.AuthenticatedData = g.AD
	m.CheckingDisabled = g.CD

	m.Question = make([]dns.Question, 1)
	m.Answer = make([]dns.RR, len(g.Answer))
	m.Ns = make([]dns.RR, len(g.Authority))
	m.Extra = make([]dns.RR, len(g.Additional))

	m.Question[0] = dns.Question{Name: g.Question[0].Name, Qtype: g.Question[0].Type, Qclass: dns.ClassINET}

	var err error
	for i := 0; i < len(m.Answer); i++ {
		m.Answer[i], err = toRR(g.Answer[i])
		if err != nil {
			return nil, err
		}
	}
	for i := 0; i < len(m.Ns); i++ {
		m.Ns[i], err = toRR(g.Authority[i])
		if err != nil {
			return nil, err
		}
	}
	for i := 0; i < len(m.Extra); i++ {
		m.Extra[i], err = toRR(g.Additional[i])
		if err != nil {
			return nil, err
		}
	}

	return m, nil
}

func toRR(g googleRR) (dns.RR, error) {
	typ, ok := dns.TypeToString[g.Type]
	if !ok {
		return nil, fmt.Errorf("failed to convert type %q", g.Type)
	}

	str := fmt.Sprintf("%s %d %s %s", g.Name, g.TTL, typ, g.Data)
	rr, err := dns.NewRR(str)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %q: %s", str, err)
	}
	return rr, nil
}

// googleRR represents a dns.RR in another form.
type googleRR struct {
	Name string
	Type uint16
	TTL  uint32
	Data string
}

// googleMsg is a JSON representation of the dns.Msg.
type googleMsg struct {
	Status   int
	TC       bool
	RD       bool
	RA       bool
	AD       bool
	CD       bool
	Question []struct {
		Name string
		Type uint16
	}
	Answer     []googleRR
	Authority  []googleRR
	Additional []googleRR
	Comment    string
}

const (
	ghost = "dns.google.com"
)
