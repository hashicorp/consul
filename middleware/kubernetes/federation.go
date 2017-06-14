package kubernetes

import (
	"net"
	"strings"

	"github.com/coredns/coredns/middleware/etcd/msg"
)

// Federation holds TODO(...).
type Federation struct {
	name string
	zone string
}

var localNodeName string
var federationZone string
var federationRegion string

const (
	// TODO: Do not hardcode these labels. Pull them out of the API instead.
	//
	// We can get them via ....
	//   import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//     metav1.LabelZoneFailureDomain
	//     metav1.LabelZoneRegion
	//
	// But importing above breaks coredns with flag collision of 'log_dir'

	labelAvailabilityZone = "failure-domain.beta.kubernetes.io/zone"
	labelRegion           = "failure-domain.beta.kubernetes.io/region"
)

// stripFederation removes the federation segment from the segment list, if it
// matches a configured federation name.
func (k *Kubernetes) stripFederation(segs []string) (string, []string) {

	if len(segs) < 3 {
		return "", segs
	}
	for _, f := range k.Federations {
		if f.name == segs[len(segs)-2] {
			fed := segs[len(segs)-2]
			segs[len(segs)-2] = segs[len(segs)-1]
			segs = segs[:len(segs)-1]
			return fed, segs
		}
	}
	return "", segs
}

// federationCNAMERecord returns a service record for the requested federated service
// with the target host in the federated CNAME format which the external DNS provider
// should be able to resolve
func (k *Kubernetes) federationCNAMERecord(r recordRequest) msg.Service {

	myNodeName := k.localNodeName()
	node, err := k.APIConn.GetNodeByName(myNodeName)
	if err != nil {
		return msg.Service{}
	}

	for _, f := range k.Federations {
		if f.name != r.federation {
			continue
		}
		if r.endpoint == "" {
			return msg.Service{
				Key:  strings.Join([]string{msg.Path(r.zone, "coredns"), r.typeName, r.federation, r.namespace, r.service}, "/"),
				Host: strings.Join([]string{r.service, r.namespace, r.federation, r.typeName, node.Labels[labelAvailabilityZone], node.Labels[labelRegion], f.zone}, "."),
			}
		}
		return msg.Service{
			Key:  strings.Join([]string{msg.Path(r.zone, "coredns"), r.typeName, r.federation, r.namespace, r.service, r.endpoint}, "/"),
			Host: strings.Join([]string{r.endpoint, r.service, r.namespace, r.federation, r.typeName, node.Labels[labelAvailabilityZone], node.Labels[labelRegion], f.zone}, "."),
		}
	}

	return msg.Service{}
}

func (k *Kubernetes) localNodeName() string {
	if localNodeName != "" {
		return localNodeName
	}
	localIP := k.localPodIP()
	if localIP == nil {
		return ""
	}
	// Find endpoint matching localIP
	endpointsList := k.APIConn.EndpointsList()
	for _, ep := range endpointsList.Items {
		for _, eps := range ep.Subsets {
			for _, addr := range eps.Addresses {
				if localIP.Equal(net.ParseIP(addr.IP)) {
					localNodeName = *addr.NodeName
					return localNodeName
				}
			}
		}
	}
	return ""
}
