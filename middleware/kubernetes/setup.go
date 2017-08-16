package kubernetes

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/middleware"
	"github.com/coredns/coredns/middleware/pkg/dnsutil"
	"github.com/coredns/coredns/middleware/proxy"
	"github.com/miekg/dns"

	"github.com/mholt/caddy"
	unversionedapi "k8s.io/client-go/1.5/pkg/api/unversioned"
)

func init() {
	caddy.RegisterPlugin("kubernetes", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	kubernetes, err := kubernetesParse(c)
	if err != nil {
		return middleware.Error("kubernetes", err)
	}

	err = kubernetes.InitKubeCache()
	if err != nil {
		return middleware.Error("kubernetes", err)
	}

	// Register KubeCache start and stop functions with Caddy
	c.OnStartup(func() error {
		go kubernetes.APIConn.Run()
		if kubernetes.APIProxy != nil {
			go kubernetes.APIProxy.Run()
		}
		return nil
	})

	c.OnShutdown(func() error {
		if kubernetes.APIProxy != nil {
			kubernetes.APIProxy.Stop()
		}
		return kubernetes.APIConn.Stop()
	})

	dnsserver.GetConfig(c).AddMiddleware(func(next middleware.Handler) middleware.Handler {
		kubernetes.Next = next
		return kubernetes
	})

	// Also register kubernetes for use in autopath.
	dnsserver.GetConfig(c).RegisterHandler(kubernetes)

	return nil
}

func kubernetesParse(c *caddy.Controller) (*Kubernetes, error) {
	k8s := &Kubernetes{
		ResyncPeriod:       defaultResyncPeriod,
		interfaceAddrsFunc: localPodIP,
		PodMode:            PodModeDisabled,
		Proxy:              proxy.Proxy{},
	}

	k8s.autoPathSearch = searchFromResolvConf()

	for c.Next() {
		zones := c.RemainingArgs()

		if len(zones) != 0 {
			k8s.Zones = zones
			for i := 0; i < len(k8s.Zones); i++ {
				k8s.Zones[i] = middleware.Host(k8s.Zones[i]).Normalize()
			}
		} else {
			k8s.Zones = make([]string, len(c.ServerBlockKeys))
			for i := 0; i < len(c.ServerBlockKeys); i++ {
				k8s.Zones[i] = middleware.Host(c.ServerBlockKeys[i]).Normalize()
			}
		}

		k8s.primaryZoneIndex = -1
		for i, z := range k8s.Zones {
			if strings.HasSuffix(z, "in-addr.arpa.") || strings.HasSuffix(z, "ip6.arpa.") {
				continue
			}
			k8s.primaryZoneIndex = i
			break
		}

		if k8s.primaryZoneIndex == -1 {
			return nil, errors.New("non-reverse zone name must be used")
		}

		for c.NextBlock() {
			switch c.Val() {
			case "pods":
				args := c.RemainingArgs()
				if len(args) == 1 {
					switch args[0] {
					case PodModeDisabled, PodModeInsecure, PodModeVerified:
						k8s.PodMode = args[0]
					default:
						return nil, fmt.Errorf("wrong value for pods: %s,  must be one of: disabled, verified, insecure", args[0])
					}
					continue
				}
				return nil, c.ArgErr()
			case "namespaces":
				args := c.RemainingArgs()
				if len(args) > 0 {
					k8s.Namespaces = append(k8s.Namespaces, args...)
					continue
				}
				return nil, c.ArgErr()
			case "endpoint":
				args := c.RemainingArgs()
				if len(args) > 0 {
					for _, endpoint := range strings.Split(args[0], ",") {
						k8s.APIServerList = append(k8s.APIServerList, strings.TrimSpace(endpoint))
					}
					continue
				}
				return nil, c.ArgErr()
			case "tls": // cert key cacertfile
				args := c.RemainingArgs()
				if len(args) == 3 {
					k8s.APIClientCert, k8s.APIClientKey, k8s.APICertAuth = args[0], args[1], args[2]
					continue
				}
				return nil, c.ArgErr()
			case "resyncperiod":
				args := c.RemainingArgs()
				if len(args) > 0 {
					rp, err := time.ParseDuration(args[0])
					if err != nil {
						return nil, fmt.Errorf("unable to parse resync duration value: '%v': %v", args[0], err)
					}
					k8s.ResyncPeriod = rp
					continue
				}
				return nil, c.ArgErr()
			case "labels":
				args := c.RemainingArgs()
				if len(args) > 0 {
					labelSelectorString := strings.Join(args, " ")
					ls, err := unversionedapi.ParseToLabelSelector(labelSelectorString)
					if err != nil {
						return nil, fmt.Errorf("unable to parse label selector value: '%v': %v", labelSelectorString, err)
					}
					k8s.LabelSelector = ls
					continue
				}
				return nil, c.ArgErr()
			case "fallthrough":
				args := c.RemainingArgs()
				if len(args) == 0 {
					k8s.Fallthrough = true
					continue
				}
				return nil, c.ArgErr()
			case "upstream":
				args := c.RemainingArgs()
				if len(args) == 0 {
					return nil, c.ArgErr()
				}
				ups, err := dnsutil.ParseHostPortOrFile(args...)
				if err != nil {
					return nil, err
				}
				k8s.Proxy = proxy.NewLookup(ups)
			default:
				return nil, c.Errf("unknown property '%s'", c.Val())
			}
		}
		return k8s, nil
	}
	return nil, errors.New("kubernetes setup called without keyword 'kubernetes' in Corefile")
}

func searchFromResolvConf() []string {
	rc, err := dns.ClientConfigFromFile("/etc/resolv.conf")
	if err != nil {
		return nil
	}
	middleware.Zones(rc.Search).Normalize()
	return rc.Search
}

const defaultResyncPeriod = 5 * time.Minute
