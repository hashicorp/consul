// Code generated by protoc-gen-deepcopy. DO NOT EDIT.
package catalogv2beta1

import (
	proto "google.golang.org/protobuf/proto"
)

// DeepCopyInto supports using ServiceEndpoints within kubernetes types, where deepcopy-gen is used.
func (in *ServiceEndpoints) DeepCopyInto(out *ServiceEndpoints) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceEndpoints. Required by controller-gen.
func (in *ServiceEndpoints) DeepCopy() *ServiceEndpoints {
	if in == nil {
		return nil
	}
	out := new(ServiceEndpoints)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new ServiceEndpoints. Required by controller-gen.
func (in *ServiceEndpoints) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using Endpoint within kubernetes types, where deepcopy-gen is used.
func (in *Endpoint) DeepCopyInto(out *Endpoint) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Endpoint. Required by controller-gen.
func (in *Endpoint) DeepCopy() *Endpoint {
	if in == nil {
		return nil
	}
	out := new(Endpoint)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new Endpoint. Required by controller-gen.
func (in *Endpoint) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}
