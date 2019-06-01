package object

import (
	api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Pod is a stripped down api.Pod with only the items we need for CoreDNS.
type Pod struct {
	// Don't add new fields to this struct without talking to the CoreDNS maintainers.
	Version   string
	PodIP     string
	Name      string
	Namespace string

	*Empty
}

// ToPod converts an api.Pod to a *Pod.
func ToPod(obj interface{}) interface{} {
	pod, ok := obj.(*api.Pod)
	if !ok {
		return nil
	}

	p := &Pod{
		Version:   pod.GetResourceVersion(),
		PodIP:     pod.Status.PodIP,
		Namespace: pod.GetNamespace(),
		Name:      pod.GetName(),
	}
	// don't add pods that are being deleted.
	t := pod.ObjectMeta.DeletionTimestamp
	if t != nil && !(*t).Time.IsZero() {
		return nil
	}

	*pod = api.Pod{}

	return p
}

var _ runtime.Object = &Pod{}

// DeepCopyObject implements the ObjectKind interface.
func (p *Pod) DeepCopyObject() runtime.Object {
	p1 := &Pod{
		Version:   p.Version,
		PodIP:     p.PodIP,
		Namespace: p.Namespace,
		Name:      p.Name,
	}
	return p1
}

// GetNamespace implements the metav1.Object interface.
func (p *Pod) GetNamespace() string { return p.Namespace }

// SetNamespace implements the metav1.Object interface.
func (p *Pod) SetNamespace(namespace string) {}

// GetName implements the metav1.Object interface.
func (p *Pod) GetName() string { return p.Name }

// SetName implements the metav1.Object interface.
func (p *Pod) SetName(name string) {}

// GetResourceVersion implements the metav1.Object interface.
func (p *Pod) GetResourceVersion() string { return p.Version }

// SetResourceVersion implements the metav1.Object interface.
func (p *Pod) SetResourceVersion(version string) {}
