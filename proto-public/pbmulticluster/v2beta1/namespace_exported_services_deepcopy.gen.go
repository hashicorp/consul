// Code generated by protoc-gen-deepcopy. DO NOT EDIT.
package multiclusterv2beta1

import (
	proto "google.golang.org/protobuf/proto"
)

// DeepCopyInto supports using NamespaceExportedServices within kubernetes types, where deepcopy-gen is used.
func (in *NamespaceExportedServices) DeepCopyInto(out *NamespaceExportedServices) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NamespaceExportedServices. Required by controller-gen.
func (in *NamespaceExportedServices) DeepCopy() *NamespaceExportedServices {
	if in == nil {
		return nil
	}
	out := new(NamespaceExportedServices)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new NamespaceExportedServices. Required by controller-gen.
func (in *NamespaceExportedServices) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}
