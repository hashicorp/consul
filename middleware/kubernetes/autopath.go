package kubernetes

import (
	"fmt"

	"github.com/coredns/coredns/request"

	"k8s.io/client-go/1.5/pkg/api"
)

func (k *Kubernetes) AutoPath(state request.Request) ([]string, error) {
	ip := state.IP()

	pod := k.PodWithIP(ip)
	if pod == nil {
		return nil, fmt.Errorf("kubernets: no pod found for %s", ip)
	}

	// something something namespace
	namespace := pod.Namespace

	search := []string{namespace} // TODO: way more

	search = append(search, "") // sentinal
	return search, nil
}

// PodWithIP return the api.Pod for source IP ip. It return nil if nothing can be found.
func (k *Kubernetes) PodWithIP(ip string) (p *api.Pod) {
	objList := k.APIConn.PodIndex(ip)
	for _, o := range objList {
		p, ok := o.(*api.Pod)
		if !ok {
			return nil
		}
		return p
	}
	return nil
}
