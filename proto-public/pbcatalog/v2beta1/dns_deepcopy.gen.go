// Code generated by protoc-gen-deepcopy. DO NOT EDIT.
package catalogv2beta1

import (
	proto "google.golang.org/protobuf/proto"
)

// DeepCopyInto supports using DNSPolicy within kubernetes types, where deepcopy-gen is used.
func (in *DNSPolicy) DeepCopyInto(out *DNSPolicy) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DNSPolicy. Required by controller-gen.
func (in *DNSPolicy) DeepCopy() *DNSPolicy {
	if in == nil {
		return nil
	}
	out := new(DNSPolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new DNSPolicy. Required by controller-gen.
func (in *DNSPolicy) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using Weights within kubernetes types, where deepcopy-gen is used.
func (in *Weights) DeepCopyInto(out *Weights) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Weights. Required by controller-gen.
func (in *Weights) DeepCopy() *Weights {
	if in == nil {
		return nil
	}
	out := new(Weights)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new Weights. Required by controller-gen.
func (in *Weights) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}
