// Code generated by protoc-gen-deepcopy. DO NOT EDIT.
package authv2beta1

import (
	proto "google.golang.org/protobuf/proto"
)

// DeepCopyInto supports using ComputedTrafficPermissions within kubernetes types, where deepcopy-gen is used.
func (in *ComputedTrafficPermissions) DeepCopyInto(out *ComputedTrafficPermissions) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ComputedTrafficPermissions. Required by controller-gen.
func (in *ComputedTrafficPermissions) DeepCopy() *ComputedTrafficPermissions {
	if in == nil {
		return nil
	}
	out := new(ComputedTrafficPermissions)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new ComputedTrafficPermissions. Required by controller-gen.
func (in *ComputedTrafficPermissions) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}
