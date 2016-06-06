// Package kubernetes provides the kubernetes backend.
package kubernetes

import (
    "fmt"
	"strings"
	"time"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/kubernetes/msg"
	k8sc "github.com/miekg/coredns/middleware/kubernetes/k8sclient"
	"github.com/miekg/coredns/middleware/proxy"
//	"github.com/miekg/coredns/middleware/singleflight"

    "github.com/miekg/dns"
	"golang.org/x/net/context"
)

type Kubernetes struct {
	Next       middleware.Handler
	Zones      []string
	Proxy      proxy.Proxy // Proxy for looking up names during the resolution process
	Ctx        context.Context
//	Inflight   *singleflight.Group
    APIConn    *k8sc.K8sConnector
}


func (g Kubernetes) getZoneForName(name string) (string, []string) {
    /*
     * getZoneForName returns the zone string that matches the name and a
     * list of the DNS labels from name that are within the zone.
     * For example, if "coredns.local" is a zone configured for the
     * Kubernetes middleware, then getZoneForName("a.b.coredns.local")
     * will return ("coredns.local", ["a", "b"]).
     */
    var zone string
    var serviceSegments []string

    for _, z := range g.Zones {
        if dns.IsSubDomain(z, name) {
            zone = z 
    
            serviceSegments = dns.SplitDomainName(name)
            serviceSegments = serviceSegments[:len(serviceSegments) - dns.CountLabel(zone)]
            break
        }
    }   

    return zone, serviceSegments
} 


// Records looks up services in kubernetes.
// If exact is true, it will lookup just
// this name. This is used when find matches when completing SRV lookups
// for instance.
func (g Kubernetes) Records(name string, exact bool) ([]msg.Service, error) {

    fmt.Println("enter Records('", name, "', ", exact, ")") 

    zone, serviceSegments := g.getZoneForName(name)

    var serviceName string
    var namespace string

    // For initial implementation, assume namespace is first serviceSegment
    // and service name is remaining segments.
    serviceSegLen := len(serviceSegments)
    if serviceSegLen >= 2 {
        namespace = serviceSegments[serviceSegLen-1]
        serviceName = strings.Join(serviceSegments[:serviceSegLen-1], ".")
    }
    // else we are looking up the zone. So handle the NS, SOA records etc.

    fmt.Println("[debug] zone: ", zone)
    fmt.Println("[debug] servicename: ", serviceName)
    fmt.Println("[debug] namespace: ", namespace)
    fmt.Println("[debug] APIconn: ", g.APIConn)

    k8sItem := g.APIConn.GetServiceItemInNamespace(namespace, serviceName)
    fmt.Println("[debug] k8s item:", k8sItem)

    switch {
        case exact && k8sItem == nil:
            fmt.Println("here2")
            return nil, nil
    }

    if k8sItem == nil {
        // Did not find item in k8s
        return nil, nil
    }

    fmt.Println("[debug] clusterIP:", k8sItem.Spec.ClusterIP)

    for _, p := range k8sItem.Spec.Ports {
        fmt.Println("[debug]    host:", name)
        fmt.Println("[debug]    port:", p.Port)
    }

    clusterIP := k8sItem.Spec.ClusterIP
    var records []msg.Service
    for _, p := range k8sItem.Spec.Ports{
        s := msg.Service{Host: clusterIP, Port: p.Port}
        records = append(records, s)
    }

    return records, nil
}

/*
// Get performs the call to the Kubernetes http API.
func (g Kubernetes) Get(path string, recursive bool) (bool, error) {

    fmt.Println("[debug] in Get path: ", path)
    fmt.Println("[debug] in Get recursive: ", recursive)

	return false, nil
}
*/

func (g Kubernetes) splitDNSName(name string) []string {
    l := dns.SplitDomainName(name)

    for i, j := 0, len(l)-1; i < j; i, j = i+1, j-1 {
        l[i], l[j] = l[j], l[i]
    }

    return l
}

// skydns/local/skydns/east/staging/web
// skydns/local/skydns/west/production/web
//
// skydns/local/skydns/*/*/web
// skydns/local/skydns/*/web

// loopNodes recursively loops through the nodes and returns all the values. The nodes' keyname
// will be match against any wildcards when star is true.
/*
func (g Kubernetes) loopNodes(ns []*etcdc.Node, nameParts []string, star bool, bx map[msg.Service]bool) (sx []msg.Service, err error) {
	if bx == nil {
		bx = make(map[msg.Service]bool)
	}
Nodes:
	for _, n := range ns {
		if n.Dir {
			nodes, err := g.loopNodes(n.Nodes, nameParts, star, bx)
			if err != nil {
				return nil, err
			}
			sx = append(sx, nodes...)
			continue
		}
		if star {
			keyParts := strings.Split(n.Key, "/")
			for i, n := range nameParts {
				if i > len(keyParts)-1 {
					// name is longer than key
					continue Nodes
				}
				if n == "*" || n == "any" {
					continue
				}
				if keyParts[i] != n {
					continue Nodes
				}
			}
		}
		serv := new(msg.Service)
		if err := json.Unmarshal([]byte(n.Value), serv); err != nil {
			return nil, err
		}
		b := msg.Service{Host: serv.Host, Port: serv.Port, Priority: serv.Priority, Weight: serv.Weight, Text: serv.Text, Key: n.Key}
		if _, ok := bx[b]; ok {
			continue
		}
		bx[b] = true

		serv.Key = n.Key
		serv.Ttl = g.Ttl(n, serv)
		if serv.Priority == 0 {
			serv.Priority = priority
		}
		sx = append(sx, *serv)
	}
	return sx, nil
}

// Ttl returns the smaller of the kubernetes TTL and the service's
// TTL. If neither of these are set (have a zero value), a default is used.
func (g Kubernetes) Ttl(node *etcdc.Node, serv *msg.Service) uint32 {
	kubernetesTtl := uint32(node.TTL)

	if kubernetesTtl == 0 && serv.Ttl == 0 {
		return ttl
	}
	if kubernetesTtl == 0 {
		return serv.Ttl
	}
	if serv.Ttl == 0 {
		return kubernetesTtl
	}
	if kubernetesTtl < serv.Ttl {
		return kubernetesTtl
	}
	return serv.Ttl
}
*/

// kubernetesNameError checks if the error is ErrorCodeKeyNotFound from kubernetes.
func isKubernetesNameError(err error) bool {
	return false
}

const (
	priority    = 10  // default priority when nothing is set
	ttl         = 300 // default ttl when nothing is set
	minTtl      = 60
	hostmaster  = "hostmaster"
	k8sTimeout = 5 * time.Second
)
