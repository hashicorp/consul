// Code generated by protoc-gen-deepcopy. DO NOT EDIT.
package catalogv2beta1

import (
	proto "google.golang.org/protobuf/proto"
)

// DeepCopyInto supports using VirtualIPs within kubernetes types, where deepcopy-gen is used.
func (in *VirtualIPs) DeepCopyInto(out *VirtualIPs) {
	p := proto.Clone(in).(*VirtualIPs)
	*out = *p
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VirtualIPs. Required by controller-gen.
func (in *VirtualIPs) DeepCopy() *VirtualIPs {
	if in == nil {
		return nil
	}
	out := new(VirtualIPs)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new VirtualIPs. Required by controller-gen.
func (in *VirtualIPs) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using IP within kubernetes types, where deepcopy-gen is used.
func (in *IP) DeepCopyInto(out *IP) {
	p := proto.Clone(in).(*IP)
	*out = *p
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IP. Required by controller-gen.
func (in *IP) DeepCopy() *IP {
	if in == nil {
		return nil
	}
	out := new(IP)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new IP. Required by controller-gen.
func (in *IP) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}
