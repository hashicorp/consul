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
	unversionedapi "k8s.io/kubernetes/pkg/api/unversioned"
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
		return err
	}

	err = kubernetes.InitKubeCache()
	if err != nil {
		return err
	}

	// Register KubeCache start and stop functions with Caddy
	c.OnStartup(func() error {
		go kubernetes.APIConn.Run()
		return nil
	})

	c.OnShutdown(func() error {
		return kubernetes.APIConn.Stop()
	})

	dnsserver.GetConfig(c).AddMiddleware(func(next dnsserver.Handler) dnsserver.Handler {
		kubernetes.Next = next
		return kubernetes
	})

	return nil
}

func kubernetesParse(c *caddy.Controller) (Kubernetes, error) {
	var err error
	template := defaultNameTemplate

	k8s := Kubernetes{ResyncPeriod: defaultResyncPeriod}
	k8s.NameTemplate = new(nametemplate.NameTemplate)
	k8s.NameTemplate.SetTemplate(template)

	for c.Next() {
		if c.Val() == "kubernetes" {
			zones := c.RemainingArgs()

			if len(zones) == 0 {
				k8s.Zones = make([]string, len(c.ServerBlockKeys))
				copy(k8s.Zones, c.ServerBlockKeys)
			}

			k8s.Zones = NormalizeZoneList(zones)
			middleware.Zones(k8s.Zones).FullyQualify()

			if k8s.Zones == nil || len(k8s.Zones) < 1 {
				err = errors.New("Zone name must be provided for kubernetes middleware.")
				return Kubernetes{}, err
			}

			for c.NextBlock() {
				switch c.Val() {
				case "template":
					args := c.RemainingArgs()
					if len(args) != 0 {
						template := strings.Join(args, "")
						err = k8s.NameTemplate.SetTemplate(template)
						if err != nil {
							return Kubernetes{}, err
						}
					} else {
						return Kubernetes{}, c.ArgErr()
					}
				case "namespaces":
					args := c.RemainingArgs()
					if len(args) != 0 {
						k8s.Namespaces = append(k8s.Namespaces, args...)
					} else {
						return Kubernetes{}, c.ArgErr()
					}
				case "endpoint":
					args := c.RemainingArgs()
					if len(args) != 0 {
						k8s.APIEndpoint = args[0]
					} else {
						return Kubernetes{}, c.ArgErr()
					}
				case "resyncperiod":
					args := c.RemainingArgs()
					if len(args) != 0 {
						k8s.ResyncPeriod, err = time.ParseDuration(args[0])
						if err != nil {
							err = errors.New(fmt.Sprintf("Unable to parse resync duration value. Value provided was '%v'. Example valid values: '15s', '5m', '1h'. Error was: %v", args[0], err))
							return Kubernetes{}, err
						}
					} else {
						return Kubernetes{}, c.ArgErr()
					}
				case "labels":
					args := c.RemainingArgs()
					if len(args) != 0 {
						labelSelectorString := strings.Join(args, " ")
						k8s.LabelSelector, err = unversionedapi.ParseToLabelSelector(labelSelectorString)
						if err != nil {
							err = errors.New(fmt.Sprintf("Unable to parse label selector. Value provided was '%v'. Error was: %v", labelSelectorString, err))
							return Kubernetes{}, err
						}
					} else {
						return Kubernetes{}, c.ArgErr()
					}
				}
			}
			return k8s, nil
		}
	}
	err = errors.New("Kubernetes setup called without keyword 'kubernetes' in Corefile")
	return Kubernetes{}, err
}

const (
	defaultNameTemplate = "{service}.{namespace}.{zone}"
	defaultResyncPeriod = 5 * time.Minute
)
