package setup

import (
	//"crypto/tls"
	//"crypto/x509"
	"fmt"
	//"io/ioutil"
	//"net"
	//"net/http"
	"strings"
	//"time"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/kubernetes"
	k8sc "github.com/miekg/coredns/middleware/kubernetes/k8sclient"
	"github.com/miekg/coredns/middleware/kubernetes/nametemplate"
	"github.com/miekg/coredns/middleware/proxy"
	//"github.com/miekg/coredns/middleware/singleflight"

	"golang.org/x/net/context"
)

const (
	defaultK8sEndpoint  = "http://localhost:8080"
	defaultNameTemplate = "{service}.{namespace}.{zone}"
)

// Kubernetes sets up the kubernetes middleware.
func Kubernetes(c *Controller) (middleware.Middleware, error) {
	fmt.Println("controller %v", c)
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

	/*
	 * TODO: Remove unused state and simplify.
	 * Inflight and Ctx might not be needed. Leaving in place until
	 * we take a pass at API caching and optimizing connector to the
	 * k8s API. Single flight (or limited upper-bound) for inflight
	 * API calls may be desirable.
	 */

	k8s := kubernetes.Kubernetes{
		Proxy: proxy.New([]string{}),
		Ctx:   context.Background(),
		//      Inflight:   &singleflight.Group{},
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

			middleware.Zones(k8s.Zones).FullyQualify()
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
						k8s.Namespaces = &namespaces
					}
				}
			}
			return k8s, nil
		}
	}
	fmt.Println("here before return")
	return kubernetes.Kubernetes{}, nil
}
