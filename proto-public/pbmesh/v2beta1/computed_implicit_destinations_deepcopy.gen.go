// Code generated by protoc-gen-deepcopy. DO NOT EDIT.
package meshv2beta1

import (
	proto "google.golang.org/protobuf/proto"
)

// DeepCopyInto supports using ComputedImplicitDestinations within kubernetes types, where deepcopy-gen is used.
func (in *ComputedImplicitDestinations) DeepCopyInto(out *ComputedImplicitDestinations) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ComputedImplicitDestinations. Required by controller-gen.
func (in *ComputedImplicitDestinations) DeepCopy() *ComputedImplicitDestinations {
	if in == nil {
		return nil
	}
	out := new(ComputedImplicitDestinations)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new ComputedImplicitDestinations. Required by controller-gen.
func (in *ComputedImplicitDestinations) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using ImplicitDestination within kubernetes types, where deepcopy-gen is used.
func (in *ImplicitDestination) DeepCopyInto(out *ImplicitDestination) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ImplicitDestination. Required by controller-gen.
func (in *ImplicitDestination) DeepCopy() *ImplicitDestination {
	if in == nil {
		return nil
	}
	out := new(ImplicitDestination)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new ImplicitDestination. Required by controller-gen.
func (in *ImplicitDestination) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}
