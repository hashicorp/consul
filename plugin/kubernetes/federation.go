package kubernetes

import (
	"github.com/coredns/coredns/plugin/etcd/msg"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/coredns/coredns/request"
)

// The federation node.Labels keys used.
const (
	// TODO: Do not hardcode these labels. Pull them out of the API instead.
	//
	// We can get them via ....
	//   import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//     metav1.LabelZoneFailureDomain
	//     metav1.LabelZoneRegion
	//
	// But importing above breaks coredns with flag collision of 'log_dir'

	LabelZone   = "failure-domain.beta.kubernetes.io/zone"
	LabelRegion = "failure-domain.beta.kubernetes.io/region"
)

// Federations is used from the federations plugin to return the service that should be
// returned as a CNAME for federation(s) to work.
func (k *Kubernetes) Federations(state request.Request, fname, fzone string) (msg.Service, error) {
	nodeName := k.localNodeName()
	node, err := k.APIConn.GetNodeByName(nodeName)
	if err != nil {
		return msg.Service{}, err
	}
	r, err := parseRequest(state)
	if err != nil {
		return msg.Service{}, err
	}

	lz := node.Labels[LabelZone]
	lr := node.Labels[LabelRegion]

	if r.endpoint == "" {
		return msg.Service{Host: dnsutil.Join([]string{r.service, r.namespace, fname, r.podOrSvc, lz, lr, fzone})}, nil
	}

	return msg.Service{Host: dnsutil.Join([]string{r.endpoint, r.service, r.namespace, fname, r.podOrSvc, lz, lr, fzone})}, nil
}
