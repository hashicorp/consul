package kubernetes

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/miekg/coredns/core/dnsserver"
	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/kubernetes/nametemplate"

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
	k8s := &Kubernetes{ResyncPeriod: defaultResyncPeriod}
	k8s.NameTemplate = new(nametemplate.Template)
	k8s.NameTemplate.SetTemplate(defaultNameTemplate)

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
				return nil, errors.New("Zone name must be provided for kubernetes middleware.")
			}

			for c.NextBlock() {
				switch c.Val() {
				case "template":
					args := c.RemainingArgs()
					if len(args) > 0 {
						template := strings.Join(args, "")
						err := k8s.NameTemplate.SetTemplate(template)
						if err != nil {
							return nil, err
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
							return nil, fmt.Errorf("Unable to parse resync duration value. Value provided was '%v'. Example valid values: '15s', '5m', '1h'. Error was: %v", args[0], err)
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
							return nil, fmt.Errorf("Unable to parse label selector. Value provided was '%v'. Error was: %v", labelSelectorString, err)
						}
						k8s.LabelSelector = ls
						continue
					}
					return nil, c.ArgErr()
				}
			}
			return k8s, nil
		}
	}
	return nil, errors.New("Kubernetes setup called without keyword 'kubernetes' in Corefile")
}

const (
	defaultNameTemplate = "{service}.{namespace}.{type}.{zone}"
	defaultResyncPeriod = 5 * time.Minute
)
