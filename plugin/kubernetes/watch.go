package kubernetes

import (
	"github.com/coredns/coredns/plugin/kubernetes/object"
	"github.com/coredns/coredns/plugin/pkg/watch"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SetWatchChan implements watch.Watchable
func (k *Kubernetes) SetWatchChan(c watch.Chan) {
	k.APIConn.SetWatchChan(c)
}

// Watch is called when a watch is started for a name.
func (k *Kubernetes) Watch(qname string) error {
	return k.APIConn.Watch(qname)
}

// StopWatching is called when no more watches remain for a name
func (k *Kubernetes) StopWatching(qname string) {
	k.APIConn.StopWatching(qname)
}

var _ watch.Watchable = &Kubernetes{}

func (dns *dnsControl) sendServiceUpdates(s *object.Service) {
	for i := range dns.zones {
		name := serviceFQDN(s, dns.zones[i])
		if _, ok := dns.watched[name]; ok {
			dns.watchChan <- name
		}
	}
}

func (dns *dnsControl) sendPodUpdates(p *object.Pod) {
	for i := range dns.zones {
		name := podFQDN(p, dns.zones[i])
		if _, ok := dns.watched[name]; ok {
			dns.watchChan <- name
		}
	}
}

func (dns *dnsControl) sendEndpointsUpdates(ep *object.Endpoints) {
	for _, zone := range dns.zones {
		for _, name := range endpointFQDN(ep, zone, dns.endpointNameMode) {
			if _, ok := dns.watched[name]; ok {
				dns.watchChan <- name
			}
		}
		name := serviceFQDN(ep, zone)
		if _, ok := dns.watched[name]; ok {
			dns.watchChan <- name
		}
	}
}

// endpointsSubsetDiffs returns an Endpoints struct containing the Subsets that have changed between a and b.
// When we notify clients of changed endpoints we only want to notify them of endpoints that have changed.
// The Endpoints API object holds more than one endpoint, held in a list of Subsets.  Each Subset refers to
// an endpoint.  So, here we create a new Endpoints struct, and populate it with only the endpoints that have changed.
// This new Endpoints object is later used to generate the list of endpoint FQDNs to send to the client.
// This function computes this literally by combining the sets (in a and not in b) union (in b and not in a).
func endpointsSubsetDiffs(a, b *object.Endpoints) *object.Endpoints {
	c := b.CopyWithoutSubsets()

	// In the following loop, the first iteration computes (in a but not in b).
	// The second iteration then adds (in b but not in a)
	// The end result is an Endpoints that only contains the subsets (endpoints) that are different between a and b.
	for _, abba := range [][]*object.Endpoints{{a, b}, {b, a}} {
		a := abba[0]
		b := abba[1]
	left:
		for _, as := range a.Subsets {
			for _, bs := range b.Subsets {
				if subsetsEquivalent(as, bs) {
					continue left
				}
			}
			c.Subsets = append(c.Subsets, as)
		}
	}
	return c
}

// sendUpdates sends a notification to the server if a watch is enabled for the qname.
func (dns *dnsControl) sendUpdates(oldObj, newObj interface{}) {
	// If both objects have the same resource version, they are identical.
	if newObj != nil && oldObj != nil && (oldObj.(meta.Object).GetResourceVersion() == newObj.(meta.Object).GetResourceVersion()) {
		return
	}
	obj := newObj
	if obj == nil {
		obj = oldObj
	}
	switch ob := obj.(type) {
	case *object.Service:
		dns.updateModifed()
		if len(dns.watched) == 0 {
			return
		}
		dns.sendServiceUpdates(ob)
	case *object.Endpoints:
		if newObj == nil || oldObj == nil {
			dns.updateModifed()
			if len(dns.watched) == 0 {
				return
			}
			dns.sendEndpointsUpdates(ob)
			return
		}
		p := oldObj.(*object.Endpoints)
		// endpoint updates can come frequently, make sure it's a change we care about
		if endpointsEquivalent(p, ob) {
			return
		}
		dns.updateModifed()
		if len(dns.watched) == 0 {
			return
		}
		dns.sendEndpointsUpdates(endpointsSubsetDiffs(p, ob))
	case *object.Pod:
		dns.updateModifed()
		if len(dns.watched) == 0 {
			return
		}
		dns.sendPodUpdates(ob)
	default:
		log.Warningf("Updates for %T not supported.", ob)
	}
}

func (dns *dnsControl) Add(obj interface{})               { dns.sendUpdates(nil, obj) }
func (dns *dnsControl) Delete(obj interface{})            { dns.sendUpdates(obj, nil) }
func (dns *dnsControl) Update(oldObj, newObj interface{}) { dns.sendUpdates(oldObj, newObj) }

// subsetsEquivalent checks if two endpoint subsets are significantly equivalent
// I.e. that they have the same ready addresses, host names, ports (including protocol
// and service names for SRV)
func subsetsEquivalent(sa, sb object.EndpointSubset) bool {
	if len(sa.Addresses) != len(sb.Addresses) {
		return false
	}
	if len(sa.Ports) != len(sb.Ports) {
		return false
	}

	// in Addresses and Ports, we should be able to rely on
	// these being sorted and able to be compared
	// they are supposed to be in a canonical format
	for addr, aaddr := range sa.Addresses {
		baddr := sb.Addresses[addr]
		if aaddr.IP != baddr.IP {
			return false
		}
		if aaddr.Hostname != baddr.Hostname {
			return false
		}
	}

	for port, aport := range sa.Ports {
		bport := sb.Ports[port]
		if aport.Name != bport.Name {
			return false
		}
		if aport.Port != bport.Port {
			return false
		}
		if aport.Protocol != bport.Protocol {
			return false
		}
	}
	return true
}

// endpointsEquivalent checks if the update to an endpoint is something
// that matters to us or if they are effectively equivalent.
func endpointsEquivalent(a, b *object.Endpoints) bool {

	if len(a.Subsets) != len(b.Subsets) {
		return false
	}

	// we should be able to rely on
	// these being sorted and able to be compared
	// they are supposed to be in a canonical format
	for i, sa := range a.Subsets {
		sb := b.Subsets[i]
		if !subsetsEquivalent(sa, sb) {
			return false
		}
	}
	return true
}
