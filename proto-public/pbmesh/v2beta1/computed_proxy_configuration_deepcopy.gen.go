// Code generated by protoc-gen-deepcopy. DO NOT EDIT.
package meshv2beta1

import (
	proto "google.golang.org/protobuf/proto"
)

// DeepCopyInto supports using ComputedProxyConfiguration within kubernetes types, where deepcopy-gen is used.
func (in *ComputedProxyConfiguration) DeepCopyInto(out *ComputedProxyConfiguration) {
	p := proto.Clone(in).(*ComputedProxyConfiguration)
	*out = *p
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ComputedProxyConfiguration. Required by controller-gen.
func (in *ComputedProxyConfiguration) DeepCopy() *ComputedProxyConfiguration {
	if in == nil {
		return nil
	}
	out := new(ComputedProxyConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new ComputedProxyConfiguration. Required by controller-gen.
func (in *ComputedProxyConfiguration) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}
