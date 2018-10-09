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
