package kubernetes

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/coredns/coredns/plugin/proxy"

	"github.com/mholt/caddy"
	"github.com/miekg/dns"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	caddy.RegisterPlugin("kubernetes", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	kubernetes, initOpts, err := kubernetesParse(c)
	if err != nil {
		return plugin.Error("kubernetes", err)
	}

	err = kubernetes.initKubeCache(initOpts)
	if err != nil {
		return plugin.Error("kubernetes", err)
	}

	// Register KubeCache start and stop functions with Caddy
	c.OnStartup(func() error {
		go kubernetes.APIConn.Run()
		if kubernetes.APIProxy != nil {
			kubernetes.APIProxy.Run()
		}
		synced := false
		for synced == false {
			synced = kubernetes.APIConn.HasSynced()
			time.Sleep(100 * time.Millisecond)
		}

		return nil
	})

	c.OnShutdown(func() error {
		if kubernetes.APIProxy != nil {
			kubernetes.APIProxy.Stop()
		}
		return kubernetes.APIConn.Stop()
	})

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		kubernetes.Next = next
		return kubernetes
	})

	return nil
}

func kubernetesParse(c *caddy.Controller) (*Kubernetes, dnsControlOpts, error) {
	k8s := New([]string{""})
	k8s.interfaceAddrsFunc = localPodIP
	k8s.autoPathSearch = searchFromResolvConf()

	opts := dnsControlOpts{
		resyncPeriod: defaultResyncPeriod,
	}

	for c.Next() {
		zones := c.RemainingArgs()

		if len(zones) != 0 {
			k8s.Zones = zones
			for i := 0; i < len(k8s.Zones); i++ {
				k8s.Zones[i] = plugin.Host(k8s.Zones[i]).Normalize()
			}
		} else {
			k8s.Zones = make([]string, len(c.ServerBlockKeys))
			for i := 0; i < len(c.ServerBlockKeys); i++ {
				k8s.Zones[i] = plugin.Host(c.ServerBlockKeys[i]).Normalize()
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
			return nil, opts, errors.New("non-reverse zone name must be used")
		}

		for c.NextBlock() {
			switch c.Val() {
			case "endpoint_pod_names":
				args := c.RemainingArgs()
				if len(args) > 0 {
					return nil, opts, c.ArgErr()
				}
				k8s.endpointNameMode = true
				continue
			case "pods":
				args := c.RemainingArgs()
				if len(args) == 1 {
					switch args[0] {
					case podModeDisabled, podModeInsecure, podModeVerified:
						k8s.podMode = args[0]
					default:
						return nil, opts, fmt.Errorf("wrong value for pods: %s,  must be one of: disabled, verified, insecure", args[0])
					}
					continue
				}
				return nil, opts, c.ArgErr()
			case "namespaces":
				args := c.RemainingArgs()
				if len(args) > 0 {
					for _, a := range args {
						k8s.Namespaces[a] = true
					}
					continue
				}
				return nil, opts, c.ArgErr()
			case "endpoint":
				args := c.RemainingArgs()
				if len(args) > 0 {
					k8s.APIServerList = args
					continue
				}
				return nil, opts, c.ArgErr()
			case "tls": // cert key cacertfile
				args := c.RemainingArgs()
				if len(args) == 3 {
					k8s.APIClientCert, k8s.APIClientKey, k8s.APICertAuth = args[0], args[1], args[2]
					continue
				}
				return nil, opts, c.ArgErr()
			case "resyncperiod":
				args := c.RemainingArgs()
				if len(args) > 0 {
					rp, err := time.ParseDuration(args[0])
					if err != nil {
						return nil, opts, fmt.Errorf("unable to parse resync duration value: '%v': %v", args[0], err)
					}
					opts.resyncPeriod = rp
					continue
				}
				return nil, opts, c.ArgErr()
			case "labels":
				args := c.RemainingArgs()
				if len(args) > 0 {
					labelSelectorString := strings.Join(args, " ")
					ls, err := meta.ParseToLabelSelector(labelSelectorString)
					if err != nil {
						return nil, opts, fmt.Errorf("unable to parse label selector value: '%v': %v", labelSelectorString, err)
					}
					opts.labelSelector = ls
					continue
				}
				return nil, opts, c.ArgErr()
			case "fallthrough":
				k8s.Fall.SetZonesFromArgs(c.RemainingArgs())
			case "upstream":
				args := c.RemainingArgs()
				if len(args) == 0 {
					return nil, opts, c.ArgErr()
				}
				ups, err := dnsutil.ParseHostPortOrFile(args...)
				if err != nil {
					return nil, opts, err
				}
				k8s.Proxy = proxy.NewLookup(ups)
			case "ttl":
				args := c.RemainingArgs()
				if len(args) == 0 {
					return nil, opts, c.ArgErr()
				}
				t, err := strconv.Atoi(args[0])
				if err != nil {
					return nil, opts, err
				}
				if t < 5 || t > 3600 {
					return nil, opts, c.Errf("ttl must be in range [5, 3600]: %d", t)
				}
				k8s.ttl = uint32(t)
			default:
				return nil, opts, c.Errf("unknown property '%s'", c.Val())
			}
		}
	}
	return k8s, opts, nil
}

func searchFromResolvConf() []string {
	rc, err := dns.ClientConfigFromFile("/etc/resolv.conf")
	if err != nil {
		return nil
	}
	plugin.Zones(rc.Search).Normalize()
	return rc.Search
}

const defaultResyncPeriod = 5 * time.Minute
