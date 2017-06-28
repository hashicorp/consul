package kubernetes

import (
	"errors"
	"fmt"
	"net"
	"strconv"
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
		return nil
	})

	c.OnShutdown(func() error {
		return kubernetes.APIConn.Stop()
	})

	dnsserver.GetConfig(c).AddMiddleware(func(next middleware.Handler) middleware.Handler {
		kubernetes.Next = next
		return kubernetes
	})

	return nil
}

func kubernetesParse(c *caddy.Controller) (*Kubernetes, error) {
	k8s := &Kubernetes{
		ResyncPeriod:   defaultResyncPeriod,
		interfaceAddrs: &interfaceAddrs{},
		PodMode:        PodModeDisabled,
	}

	for c.Next() {
		if c.Val() == "kubernetes" {
			zones := c.RemainingArgs()

			if len(zones) == 0 {
				k8s.Zones = make([]string, len(c.ServerBlockKeys))
				copy(k8s.Zones, c.ServerBlockKeys)
			}

			k8s.Zones = NormalizeZoneList(zones)
			middleware.Zones(k8s.Zones).Normalize()

			if k8s.Zones == nil || len(k8s.Zones) < 1 {
				return nil, errors.New("zone name must be provided for kubernetes middleware")
			}

			k8s.primaryZone = -1
			for i, z := range k8s.Zones {
				if strings.HasSuffix(z, "in-addr.arpa.") || strings.HasSuffix(z, "ip6.arpa.") {
					continue
				}
				k8s.primaryZone = i
				break
			}

			if k8s.primaryZone == -1 {
				return nil, errors.New("non-reverse zone name must be given for Kubernetes")
			}

			for c.NextBlock() {
				switch c.Val() {
				case "cidrs":
					args := c.RemainingArgs()
					if len(args) > 0 {
						for _, cidrStr := range args {
							_, cidr, err := net.ParseCIDR(cidrStr)
							if err != nil {
								return nil, fmt.Errorf("invalid cidr: %s", cidrStr)
							}
							k8s.ReverseCidrs = append(k8s.ReverseCidrs, *cidr)

						}
						continue
					}
					return nil, c.ArgErr()
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
						k8s.APIEndpoint = args[0]
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
				case "federation": // name zone
					args := c.RemainingArgs()
					if len(args) == 2 {
						k8s.Federations = append(k8s.Federations, Federation{
							name: args[0],
							zone: args[1],
						})
						continue
					}
					return nil, fmt.Errorf("incorrect number of arguments for federation, got %v, expected 2", len(args))
				case "autopath": // name zone
					args := c.RemainingArgs()
					k8s.AutoPath = AutoPath{
						NDots:          defautNdots,
						HostSearchPath: []string{},
						ResolvConfFile: defaultResolvConfFile,
						OnNXDOMAIN:     defaultOnNXDOMAIN,
					}
					if len(args) > 3 {
						return nil, fmt.Errorf("incorrect number of arguments for autopath, got %v, expected at most 3", len(args))

					}
					if len(args) > 0 {
						ndots, err := strconv.Atoi(args[0])
						if err != nil {
							return nil, fmt.Errorf("invalid NDOTS argument for autopath, got '%v', expected an integer", ndots)
						}
						k8s.AutoPath.NDots = ndots
					}
					if len(args) > 1 {
						switch args[1] {
						case dns.RcodeToString[dns.RcodeNameError]:
							k8s.AutoPath.OnNXDOMAIN = dns.RcodeNameError
						case dns.RcodeToString[dns.RcodeSuccess]:
							k8s.AutoPath.OnNXDOMAIN = dns.RcodeSuccess
						case dns.RcodeToString[dns.RcodeServerFailure]:
							k8s.AutoPath.OnNXDOMAIN = dns.RcodeServerFailure
						default:
							return nil, fmt.Errorf("invalid RESPONSE argument for autopath, got '%v', expected SERVFAIL, NOERROR, or NXDOMAIN", args[1])
						}
					}
					if len(args) > 2 {
						k8s.AutoPath.ResolvConfFile = args[2]
					}
					rc, err := dns.ClientConfigFromFile(k8s.AutoPath.ResolvConfFile)
					if err != nil {
						return nil, fmt.Errorf("error when parsing %v: %v", k8s.AutoPath.ResolvConfFile, err)
					}
					k8s.AutoPath.HostSearchPath = rc.Search
					middleware.Zones(k8s.AutoPath.HostSearchPath).Normalize()
					k8s.AutoPath.Enabled = true
					continue
				}
			}
			return k8s, nil
		}
	}
	return nil, errors.New("kubernetes setup called without keyword 'kubernetes' in Corefile")
}

const (
	defaultResyncPeriod   = 5 * time.Minute
	defaultPodMode        = PodModeDisabled
	defautNdots           = 0
	defaultResolvConfFile = "/etc/resolv.conf"
	defaultOnNXDOMAIN     = dns.RcodeServerFailure
)
