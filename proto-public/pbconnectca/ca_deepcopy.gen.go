// Code generated by protoc-gen-deepcopy. DO NOT EDIT.
package pbconnectca

import (
	proto "google.golang.org/protobuf/proto"
)

// DeepCopyInto supports using WatchRootsRequest within kubernetes types, where deepcopy-gen is used.
func (in *WatchRootsRequest) DeepCopyInto(out *WatchRootsRequest) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WatchRootsRequest. Required by controller-gen.
func (in *WatchRootsRequest) DeepCopy() *WatchRootsRequest {
	if in == nil {
		return nil
	}
	out := new(WatchRootsRequest)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new WatchRootsRequest. Required by controller-gen.
func (in *WatchRootsRequest) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using WatchRootsResponse within kubernetes types, where deepcopy-gen is used.
func (in *WatchRootsResponse) DeepCopyInto(out *WatchRootsResponse) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WatchRootsResponse. Required by controller-gen.
func (in *WatchRootsResponse) DeepCopy() *WatchRootsResponse {
	if in == nil {
		return nil
	}
	out := new(WatchRootsResponse)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new WatchRootsResponse. Required by controller-gen.
func (in *WatchRootsResponse) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using CARoot within kubernetes types, where deepcopy-gen is used.
func (in *CARoot) DeepCopyInto(out *CARoot) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CARoot. Required by controller-gen.
func (in *CARoot) DeepCopy() *CARoot {
	if in == nil {
		return nil
	}
	out := new(CARoot)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new CARoot. Required by controller-gen.
func (in *CARoot) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using SignRequest within kubernetes types, where deepcopy-gen is used.
func (in *SignRequest) DeepCopyInto(out *SignRequest) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SignRequest. Required by controller-gen.
func (in *SignRequest) DeepCopy() *SignRequest {
	if in == nil {
		return nil
	}
	out := new(SignRequest)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new SignRequest. Required by controller-gen.
func (in *SignRequest) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using SignResponse within kubernetes types, where deepcopy-gen is used.
func (in *SignResponse) DeepCopyInto(out *SignResponse) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SignResponse. Required by controller-gen.
func (in *SignResponse) DeepCopy() *SignResponse {
	if in == nil {
		return nil
	}
	out := new(SignResponse)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new SignResponse. Required by controller-gen.
func (in *SignResponse) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}
