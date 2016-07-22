package setup

import (
	"log"
	"strings"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/kubernetes"
	k8sc "github.com/miekg/coredns/middleware/kubernetes/k8sclient"
	"github.com/miekg/coredns/middleware/kubernetes/nametemplate"
	"github.com/miekg/coredns/middleware/proxy"
)

const (
	defaultK8sEndpoint  = "http://localhost:8080"
	defaultNameTemplate = "{service}.{namespace}.{zone}"
)

// Kubernetes sets up the kubernetes middleware.
func Kubernetes(c *Controller) (middleware.Middleware, error) {
	log.Printf("[debug] controller %v\n", c)
	// TODO: Determine if subzone support required

	kubernetes, err := kubernetesParse(c)

	if err != nil {
		return nil, err
	}

	return func(next middleware.Handler) middleware.Handler {
		kubernetes.Next = next
		return kubernetes
	}, nil
}

func kubernetesParse(c *Controller) (kubernetes.Kubernetes, error) {
	k8s := kubernetes.Kubernetes{
		Proxy: proxy.New([]string{}),
	}
	var (
		endpoints  = []string{defaultK8sEndpoint}
		template   = defaultNameTemplate
		namespaces = []string{}
	)

	k8s.APIConn = k8sc.NewK8sConnector(endpoints[0])
	k8s.NameTemplate = new(nametemplate.NameTemplate)
	k8s.NameTemplate.SetTemplate(template)

	for c.Next() {
		if c.Val() == "kubernetes" {
			zones := c.RemainingArgs()

			if len(zones) == 0 {
				k8s.Zones = c.ServerBlockHosts
			} else {
				// Normalize requested zones
				k8s.Zones = kubernetes.NormalizeZoneList(zones)
			}

			// TODO: clean this parsing up

			middleware.Zones(k8s.Zones).FullyQualify()

			log.Printf("[debug] c data: %v\n", c)

			if c.NextBlock() {
				// TODO(miek): 2 switches?
				switch c.Val() {
				case "endpoint":
					args := c.RemainingArgs()
					if len(args) == 0 {
						return kubernetes.Kubernetes{}, c.ArgErr()
					}
					endpoints = args
					k8s.APIConn = k8sc.NewK8sConnector(endpoints[0])
				case "namespaces":
					args := c.RemainingArgs()
					if len(args) == 0 {
						return kubernetes.Kubernetes{}, c.ArgErr()
					}
					namespaces = args
					k8s.Namespaces = append(k8s.Namespaces, namespaces...)
				}
				for c.Next() {
					switch c.Val() {
					case "template":
						args := c.RemainingArgs()
						if len(args) == 0 {
							return kubernetes.Kubernetes{}, c.ArgErr()
						}
						template = strings.Join(args, "")
						err := k8s.NameTemplate.SetTemplate(template)
						if err != nil {
							return kubernetes.Kubernetes{}, err
						}
					case "namespaces":
						args := c.RemainingArgs()
						if len(args) == 0 {
							return kubernetes.Kubernetes{}, c.ArgErr()
						}
						namespaces = args
						k8s.Namespaces = append(k8s.Namespaces, namespaces...)
					}
				}
			}
			return k8s, nil
		}
	}
	return kubernetes.Kubernetes{}, nil
}
