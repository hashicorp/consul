// Code generated by protoc-gen-deepcopy. DO NOT EDIT.
package authv2beta1

import (
	proto "google.golang.org/protobuf/proto"
)

// DeepCopyInto supports using WorkloadIdentity within kubernetes types, where deepcopy-gen is used.
func (in *WorkloadIdentity) DeepCopyInto(out *WorkloadIdentity) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WorkloadIdentity. Required by controller-gen.
func (in *WorkloadIdentity) DeepCopy() *WorkloadIdentity {
	if in == nil {
		return nil
	}
	out := new(WorkloadIdentity)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new WorkloadIdentity. Required by controller-gen.
func (in *WorkloadIdentity) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}
