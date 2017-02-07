package proxy

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/miekg/coredns/middleware/pkg/debug"
	"github.com/miekg/coredns/request"

	"github.com/miekg/dns"
)

type google struct {
	client *http.Client

	endpoint string // Name to resolve via 'bootstrapProxy'

	bootstrapProxy Proxy
	quit           chan bool
}

func newGoogle(endpoint string, bootstrap []string) *google {
	if endpoint == "" {
		endpoint = ghost
	}
	tls := &tls.Config{ServerName: endpoint}
	client := &http.Client{
		Timeout:   time.Second * defaultTimeout,
		Transport: &http.Transport{TLSClientConfig: tls},
	}

	boot := NewLookup(bootstrap)

	return &google{client: client, endpoint: dns.Fqdn(endpoint), bootstrapProxy: boot, quit: make(chan bool)}
}

func (g *google) Exchange(addr string, state request.Request) (*dns.Msg, error) {
	v := url.Values{}

	v.Set("name", state.Name())
	v.Set("type", fmt.Sprintf("%d", state.QType()))

	optDebug := false
	if bug := debug.IsDebug(state.Name()); bug != "" {
		optDebug = true
		v.Set("name", bug)
	}

	buf, backendErr := g.exchangeJSON(addr, v.Encode())

	if backendErr == nil {
		gm := new(googleMsg)
		if err := json.Unmarshal(buf, gm); err != nil {
			return nil, err
		}

		m, debug, err := toMsg(gm)
		if err != nil {
			return nil, err
		}

		if optDebug {
			// reset question
			m.Question[0].Name = state.QName()
			// prepend debug RR to the additional section
			m.Extra = append([]dns.RR{debug}, m.Extra...)

		}

		m.Id = state.Req.Id
		return m, nil
	}

	log.Printf("[WARNING] Failed to connect to HTTPS backend %q: %s", g.endpoint, backendErr)
	return nil, backendErr
}

func (g *google) exchangeJSON(addr, json string) ([]byte, error) {
	url := "https://" + addr + "/resolve?" + json
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Host = g.endpoint // TODO(miek): works with the extra dot at the end?

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

func (g *google) Protocol() string { return "https_google" }

func (g *google) OnShutdown(p *Proxy) error {
	g.quit <- true
	return nil
}

func (g *google) OnStartup(p *Proxy) error {
	// We fake a state because normally the proxy is called after we already got a incoming query.
	// This is a non-edns0, udp request to g.endpoint.
	req := new(dns.Msg)
	req.SetQuestion(g.endpoint, dns.TypeA)
	state := request.Request{W: new(fakeBootWriter), Req: req}

	new, err := g.bootstrapProxy.Lookup(state, g.endpoint, dns.TypeA)

	var oldUpstream Upstream

	// ignore errors here, as we want to keep on trying.
	if err != nil {
		log.Printf("[WARNING] Failed to bootstrap A records %q: %s", g.endpoint, err)
	} else {
		addrs, err1 := extractAnswer(new)
		if err1 != nil {
			log.Printf("[WARNING] Failed to bootstrap A records %q: %s", g.endpoint, err)
		}

		if len(*p.Upstreams) > 0 {
			oldUpstream = (*p.Upstreams)[0]
			up := newUpstream(addrs, oldUpstream.(*staticUpstream))
			p.Upstreams = &[]Upstream{up}
		} else {
			log.Printf("[WARNING] Failed to bootstrap upstreams %q", g.endpoint)
		}
	}

	go func() {
		tick := time.NewTicker(300 * time.Second)

		for {
			select {
			case <-tick.C:

				new, err := g.bootstrapProxy.Lookup(state, g.endpoint, dns.TypeA)
				if err != nil {
					log.Printf("[WARNING] Failed to bootstrap A records %q: %s", g.endpoint, err)
				} else {
					addrs, err1 := extractAnswer(new)
					if err1 != nil {
						log.Printf("[WARNING] Failed to bootstrap A records %q: %s", g.endpoint, err)
						continue
					}

					// TODO(miek): can this actually happen?
					if oldUpstream != nil {
						up := newUpstream(addrs, oldUpstream.(*staticUpstream))
						p.Upstreams = &[]Upstream{up}
					}
				}

			case <-g.quit:
				return
			}
		}
	}()

	return nil
}

func extractAnswer(m *dns.Msg) ([]string, error) {
	if len(m.Answer) == 0 {
		return nil, fmt.Errorf("no answer section in response")
	}
	ret := []string{}
	for _, an := range m.Answer {
		if a, ok := an.(*dns.A); ok {
			ret = append(ret, net.JoinHostPort(a.A.String(), "443"))
		}
	}
	if len(ret) > 0 {
		return ret, nil
	}

	return nil, fmt.Errorf("no address records in answer section")
}

// newUpstream returns an upstream initialized with hosts.
func newUpstream(hosts []string, old *staticUpstream) Upstream {
	upstream := &staticUpstream{
		from:              old.from,
		Hosts:             nil,
		Policy:            &Random{},
		Spray:             nil,
		FailTimeout:       10 * time.Second,
		MaxFails:          3,
		ex:                old.ex,
		WithoutPathPrefix: old.WithoutPathPrefix,
		IgnoredSubDomains: old.IgnoredSubDomains,
	}

	upstream.Hosts = make([]*UpstreamHost, len(hosts))
	for i, h := range hosts {
		uh := &UpstreamHost{
			Name:        h,
			Conns:       0,
			Fails:       0,
			FailTimeout: upstream.FailTimeout,
			Unhealthy:   false,

			CheckDown: func(upstream *staticUpstream) UpstreamHostDownFunc {
				return func(uh *UpstreamHost) bool {
					if uh.Unhealthy {
						return true
					}

					fails := atomic.LoadInt32(&uh.Fails)
					if fails >= upstream.MaxFails && upstream.MaxFails != 0 {
						return true
					}
					return false
				}
			}(upstream),
			WithoutPathPrefix: upstream.WithoutPathPrefix,
		}
		upstream.Hosts[i] = uh
	}
	return upstream
}

const (
	// Default endpoint for this service.
	ghost = "dns.google.com."
)
