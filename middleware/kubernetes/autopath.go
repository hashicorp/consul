package kubernetes

import "k8s.io/client-go/1.5/pkg/api"

// TODO(miek): rename and put in autopath.go file. This will be for the
// external middleware autopath to use. Mostly to get the namespace:
//name, path, ok := autopath.SplitSearch(zone, state.QName(), p.Namespace)
func (k *Kubernetes) findPodWithIP(ip string) (p *api.Pod) {
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
