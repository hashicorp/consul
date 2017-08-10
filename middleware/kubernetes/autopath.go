package kubernetes

import (
	"github.com/coredns/coredns/middleware"
	"github.com/coredns/coredns/request"

	"k8s.io/client-go/1.5/pkg/api"
)

// AutoPath implements the AutoPathFunc call from the autopath middleware.
// It returns a per-query search path or nil indicating no searchpathing should happen.
func (k *Kubernetes) AutoPath(state request.Request) []string {
	// Check if the query falls in a zone we are actually authoriative for and thus if we want autopath.
	zone := middleware.Zones(k.Zones).Matches(state.Name())
	if zone == "" {
		return nil
	}

	ip := state.IP()

	pod := k.podWithIP(ip)
	if pod == nil {
		return nil
	}

	search := make([]string, 3)
	if zone == "." {
		search[0] = pod.Namespace + ".svc."
		search[1] = "svc."
		search[2] = "."
	} else {
		search[0] = pod.Namespace + ".svc." + zone
		search[1] = "svc." + zone
		search[2] = zone
	}

	search = append(search, k.autoPathSearch...)
	search = append(search, "") // sentinal
	return search
}

// podWithIP return the api.Pod for source IP ip. It returns nil if nothing can be found.
func (k *Kubernetes) podWithIP(ip string) (p *api.Pod) {
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
