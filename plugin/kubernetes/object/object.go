// Package object holds functions that convert the objects from the k8s API in
// to a more memory efficient structures.
//
// Adding new fields to any of the structures defined in pod.go, endpoint.go
// and service.go should not be done lightly as this increases the memory use
// and will leads to OOMs in the k8s scale test.
//
// We can do some optimizations here as well. We store IP addresses as strings,
// this might be moved to uint32 (for v4) for instance, but then we need to
// convert those again.
//
// Also the msg.Service use in this plugin may be deprecated at some point, as
// we don't use most of those features anyway and would free us from the *etcd*
// dependency, where msg.Service is defined. And should save some mem/cpu as we
// convert to and from msg.Services.
package object

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// ToFunc converts one empty interface to another.
type ToFunc func(interface{}) interface{}

// Empty is an empty struct.
type Empty struct{}

// GetObjectKind implementss the ObjectKind interface as a noop.
func (e *Empty) GetObjectKind() schema.ObjectKind { return schema.EmptyObjectKind }

// GetGenerateName implements the metav1.Object interface.
func (e *Empty) GetGenerateName() string { return "" }

// SetGenerateName implements the metav1.Object interface.
func (e *Empty) SetGenerateName(name string) {}

// GetUID implements the metav1.Object interface.
func (e *Empty) GetUID() types.UID { return "" }

// SetUID implements the metav1.Object interface.
func (e *Empty) SetUID(uid types.UID) {}

// GetGeneration implements the metav1.Object interface.
func (e *Empty) GetGeneration() int64 { return 0 }

// SetGeneration implements the metav1.Object interface.
func (e *Empty) SetGeneration(generation int64) {}

// GetSelfLink implements the metav1.Object interface.
func (e *Empty) GetSelfLink() string { return "" }

// SetSelfLink implements the metav1.Object interface.
func (e *Empty) SetSelfLink(selfLink string) {}

// GetCreationTimestamp implements the metav1.Object interface.
func (e *Empty) GetCreationTimestamp() v1.Time { return v1.Time{} }

// SetCreationTimestamp implements the metav1.Object interface.
func (e *Empty) SetCreationTimestamp(timestamp v1.Time) {}

// GetDeletionTimestamp implements the metav1.Object interface.
func (e *Empty) GetDeletionTimestamp() *v1.Time { return &v1.Time{} }

// SetDeletionTimestamp implements the metav1.Object interface.
func (e *Empty) SetDeletionTimestamp(timestamp *v1.Time) {}

// GetDeletionGracePeriodSeconds implements the metav1.Object interface.
func (e *Empty) GetDeletionGracePeriodSeconds() *int64 { return nil }

// SetDeletionGracePeriodSeconds implements the metav1.Object interface.
func (e *Empty) SetDeletionGracePeriodSeconds(*int64) {}

// GetLabels implements the metav1.Object interface.
func (e *Empty) GetLabels() map[string]string { return nil }

// SetLabels implements the metav1.Object interface.
func (e *Empty) SetLabels(labels map[string]string) {}

// GetAnnotations implements the metav1.Object interface.
func (e *Empty) GetAnnotations() map[string]string { return nil }

// SetAnnotations implements the metav1.Object interface.
func (e *Empty) SetAnnotations(annotations map[string]string) {}

// GetInitializers implements the metav1.Object interface.
func (e *Empty) GetInitializers() *v1.Initializers { return nil }

// SetInitializers implements the metav1.Object interface.
func (e *Empty) SetInitializers(initializers *v1.Initializers) {}

// GetFinalizers implements the metav1.Object interface.
func (e *Empty) GetFinalizers() []string { return nil }

// SetFinalizers implements the metav1.Object interface.
func (e *Empty) SetFinalizers(finalizers []string) {}

// GetOwnerReferences implements the metav1.Object interface.
func (e *Empty) GetOwnerReferences() []v1.OwnerReference { return nil }

// SetOwnerReferences implements the metav1.Object interface.
func (e *Empty) SetOwnerReferences([]v1.OwnerReference) {}

// GetClusterName implements the metav1.Object interface.
func (e *Empty) GetClusterName() string { return "" }

// SetClusterName implements the metav1.Object interface.
func (e *Empty) SetClusterName(clusterName string) {}

// GetManagedFields implements the metav1.Object interface.
func (e *Empty) GetManagedFields() []v1.ManagedFieldsEntry { return nil }

// SetManagedFields implements the metav1.Object interface.
func (e *Empty) SetManagedFields(managedFields []v1.ManagedFieldsEntry) {}
