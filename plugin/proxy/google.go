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
	"time"

	"github.com/coredns/coredns/plugin/pkg/healthcheck"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
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

func (g *google) Exchange(ctx context.Context, addr string, state request.Request) (*dns.Msg, error) {
	v := url.Values{}

	v.Set("name", state.Name())
	v.Set("type", fmt.Sprintf("%d", state.QType()))

	buf, backendErr := g.exchangeJSON(addr, v.Encode())

	if backendErr == nil {
		gm := new(googleMsg)
		if err := json.Unmarshal(buf, gm); err != nil {
			return nil, err
		}

		m, err := toMsg(gm)
		if err != nil {
			return nil, err
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

func (g *google) Transport() string { return "tcp" }
func (g *google) Protocol() string  { return "https_google" }

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

	if len(*p.Upstreams) == 0 {
		return fmt.Errorf("no upstreams defined")
	}

	oldUpstream := (*p.Upstreams)[0]

	new, err := g.bootstrapProxy.Lookup(state, g.endpoint, dns.TypeA)
	if err != nil {
		log.Printf("[WARNING] Failed to bootstrap A records %q: %s", g.endpoint, err)
	} else {
		addrs, err1 := extractAnswer(new)
		if err1 != nil {
			log.Printf("[WARNING] Failed to bootstrap A records %q: %s", g.endpoint, err1)
		} else {

			up := newUpstream(addrs, oldUpstream.(*staticUpstream))
			p.Upstreams = &[]Upstream{up}

			log.Printf("[INFO] Bootstrapping A records %q found: %v", g.endpoint, addrs)
		}
	}

	go func() {
		tick := time.NewTicker(120 * time.Second)

		for {
			select {
			case <-tick.C:

				log.Printf("[INFO] Resolving A records %q", g.endpoint)

				new, err := g.bootstrapProxy.Lookup(state, g.endpoint, dns.TypeA)
				if err != nil {
					log.Printf("[WARNING] Failed to resolve A records %q: %s", g.endpoint, err)
					continue
				}

				addrs, err1 := extractAnswer(new)
				if err1 != nil {
					log.Printf("[WARNING] Failed to resolve A records %q: %s", g.endpoint, err1)
					continue
				}

				up := newUpstream(addrs, oldUpstream.(*staticUpstream))
				p.Upstreams = &[]Upstream{up}

				log.Printf("[INFO] Resolving A records %q found: %v", g.endpoint, addrs)

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
		from: old.from,
		HealthCheck: healthcheck.HealthCheck{
			FailTimeout: 5 * time.Second,
			MaxFails:    3,
		},
		ex:                old.ex,
		IgnoredSubDomains: old.IgnoredSubDomains,
	}

	upstream.Hosts = make([]*healthcheck.UpstreamHost, len(hosts))
	for i, host := range hosts {
		uh := &healthcheck.UpstreamHost{
			Name:        host,
			Conns:       0,
			Fails:       0,
			FailTimeout: upstream.FailTimeout,
			CheckDown:   checkDownFunc(upstream),
		}
		upstream.Hosts[i] = uh
	}
	return upstream
}

const (
	// Default endpoint for this service.
	ghost = "dns.google.com."
)
