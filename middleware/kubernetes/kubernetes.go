// Package kubernetes provides the kubernetes backend.
package kubernetes

import (
	"errors"
	"log"
	"time"

	"github.com/miekg/coredns/middleware"
	k8sc "github.com/miekg/coredns/middleware/kubernetes/k8sclient"
	"github.com/miekg/coredns/middleware/kubernetes/msg"
	"github.com/miekg/coredns/middleware/kubernetes/nametemplate"
	"github.com/miekg/coredns/middleware/kubernetes/util"
	"github.com/miekg/coredns/middleware/proxy"
	//	"github.com/miekg/coredns/middleware/singleflight"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

type Kubernetes struct {
	Next  middleware.Handler
	Zones []string
	Proxy proxy.Proxy // Proxy for looking up names during the resolution process
	Ctx   context.Context
	//	Inflight   *singleflight.Group
	APIConn      *k8sc.K8sConnector
	NameTemplate *nametemplate.NameTemplate
	Namespaces   *[]string
}

// getZoneForName returns the zone string that matches the name and a
// list of the DNS labels from name that are within the zone.
// For example, if "coredns.local" is a zone configured for the
// Kubernetes middleware, then getZoneForName("a.b.coredns.local")
// will return ("coredns.local", ["a", "b"]).
func (g Kubernetes) getZoneForName(name string) (string, []string) {
	var zone string
	var serviceSegments []string

	for _, z := range g.Zones {
		if dns.IsSubDomain(z, name) {
			zone = z

			serviceSegments = dns.SplitDomainName(name)
			serviceSegments = serviceSegments[:len(serviceSegments)-dns.CountLabel(zone)]
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
	var (
		serviceName string
		namespace   string
		typeName    string
	)

	log.Printf("[debug] enter Records('%v', '%v')\n", name, exact)
	zone, serviceSegments := g.getZoneForName(name)

	// TODO: Implementation above globbed together segments for the serviceName if
	//       multiple segments remained. Determine how to do similar globbing using
	//		 the template-based implementation.
	namespace = g.NameTemplate.GetNamespaceFromSegmentArray(serviceSegments)
	serviceName = g.NameTemplate.GetServiceFromSegmentArray(serviceSegments)
	typeName = g.NameTemplate.GetTypeFromSegmentArray(serviceSegments)

	if namespace == "" {
		err := errors.New("Parsing query string did not produce a namespace value. Assuming wildcard namespace.")
		log.Printf("[WARN] %v\n", err)
		namespace = util.WildcardStar
	}

	if serviceName == "" {
		err := errors.New("Parsing query string did not produce a serviceName value. Assuming wildcard serviceName.")
		log.Printf("[WARN] %v\n", err)
		serviceName = util.WildcardStar
	}

	log.Printf("[debug] exact: %v\n", exact)
	log.Printf("[debug] zone: %v\n", zone)
	log.Printf("[debug] servicename: %v\n", serviceName)
	log.Printf("[debug] namespace: %v\n", namespace)
	log.Printf("[debug] typeName: %v\n", typeName)
	log.Printf("[debug] APIconn: %v\n", g.APIConn)

	nsWildcard := util.SymbolContainsWildcard(namespace)
	serviceWildcard := util.SymbolContainsWildcard(serviceName)

	// Abort if the namespace does not contain a wildcard, and namespace is not published per CoreFile
	// Case where namespace contains a wildcard is handled in Get(...) method.
	if (!nsWildcard) && (g.Namespaces != nil && !util.StringInSlice(namespace, *g.Namespaces)) {
		log.Printf("[debug] Namespace '%v' is not published by Corefile\n", namespace)
		return nil, nil
	}

	k8sItems, err := g.Get(namespace, nsWildcard, serviceName, serviceWildcard)
	log.Printf("[debug] k8s items: %v\n", k8sItems)
	if err != nil {
		log.Printf("[ERROR] Got error while looking up ServiceItems. Error is: %v\n", err)
		return nil, err
	}
	if k8sItems == nil {
		// Did not find item in k8s
		return nil, nil
	}

	records := g.getRecordsForServiceItems(k8sItems, nametemplate.NameValues{TypeName: typeName, ServiceName: serviceName, Namespace: namespace, Zone: zone})
	return records, nil
}

// TODO: assemble name from parts found in k8s data based on name template rather than reusing query string
func (g Kubernetes) getRecordsForServiceItems(serviceItems []k8sc.ServiceItem, values nametemplate.NameValues) []msg.Service {
	var records []msg.Service

	for _, item := range serviceItems {
		clusterIP := item.Spec.ClusterIP
		log.Printf("[debug] clusterIP: %v\n", clusterIP)

		// Create records by constructing record name from template...
		//values.Namespace = item.Metadata.Namespace
		//values.ServiceName = item.Metadata.Name
		//s := msg.Service{Host: g.NameTemplate.GetRecordNameFromNameValues(values)}
		//records = append(records, s)

		// Create records for each exposed port...
		for _, p := range item.Spec.Ports {
			log.Printf("[debug]    port: %v\n", p.Port)
			s := msg.Service{Host: clusterIP, Port: p.Port}
			records = append(records, s)
		}
	}

	log.Printf("[debug] records from getRecordsForServiceItems(): %v\n", records)
	return records
}

// Get performs the call to the Kubernetes http API.
func (g Kubernetes) Get(namespace string, nsWildcard bool, servicename string, serviceWildcard bool) ([]k8sc.ServiceItem, error) {
	serviceList, err := g.APIConn.GetServiceList()

	if err != nil {
		log.Printf("[ERROR] Getting service list produced error: %v", err)
		return nil, err
	}

	var resultItems []k8sc.ServiceItem

	for _, item := range serviceList.Items {
		if symbolMatches(namespace, item.Metadata.Namespace, nsWildcard) && symbolMatches(servicename, item.Metadata.Name, serviceWildcard) {
			// If namespace has a wildcard, filter results against Corefile namespace list.
			// (Namespaces without a wildcard were filtered before the call to this function.)
			if nsWildcard && (g.Namespaces != nil && !util.StringInSlice(item.Metadata.Namespace, *g.Namespaces)) {
				log.Printf("[debug] Namespace '%v' is not published by Corefile\n", item.Metadata.Namespace)
				continue
			}
			resultItems = append(resultItems, item)
		}
	}

	return resultItems, nil
}

func symbolMatches(queryString string, candidateString string, wildcard bool) bool {
	result := false
	switch {
	case !wildcard:
		result = (queryString == candidateString)
	case queryString == util.WildcardStar:
		result = true
	case queryString == util.WildcardAny:
		result = true
	}
	return result
}

// TODO: Remove these unused functions. One is related to Ttl calculation
//       Implement Ttl and priority calculation based on service count before
//       removing this code.
/*
// splitDNSName separates the name into DNS segments and reverses the segments.
func (g Kubernetes) splitDNSName(name string) []string {
	l := dns.SplitDomainName(name)

	for i, j := 0, len(l)-1; i < j; i, j = i+1, j-1 {
		l[i], l[j] = l[j], l[i]
	}

	return l
}
*/
// skydns/local/skydns/east/staging/web
// skydns/local/skydns/west/production/web
//
// skydns/local/skydns/*/*/web
// skydns/local/skydns/*/web
/*
// loopNodes recursively loops through the nodes and returns all the values. The nodes' keyname
// will be match against any wildcards when star is true.
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
	priority   = 10  // default priority when nothing is set
	ttl        = 300 // default ttl when nothing is set
	minTtl     = 60
	hostmaster = "hostmaster"
	k8sTimeout = 5 * time.Second
)
