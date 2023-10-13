// Code generated by protoc-gen-deepcopy. DO NOT EDIT.
package pbresource

import (
	proto "google.golang.org/protobuf/proto"
)

// DeepCopyInto supports using Type within kubernetes types, where deepcopy-gen is used.
func (in *Type) DeepCopyInto(out *Type) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Type. Required by controller-gen.
func (in *Type) DeepCopy() *Type {
	if in == nil {
		return nil
	}
	out := new(Type)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new Type. Required by controller-gen.
func (in *Type) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using Tenancy within kubernetes types, where deepcopy-gen is used.
func (in *Tenancy) DeepCopyInto(out *Tenancy) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Tenancy. Required by controller-gen.
func (in *Tenancy) DeepCopy() *Tenancy {
	if in == nil {
		return nil
	}
	out := new(Tenancy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new Tenancy. Required by controller-gen.
func (in *Tenancy) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using ID within kubernetes types, where deepcopy-gen is used.
func (in *ID) DeepCopyInto(out *ID) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ID. Required by controller-gen.
func (in *ID) DeepCopy() *ID {
	if in == nil {
		return nil
	}
	out := new(ID)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new ID. Required by controller-gen.
func (in *ID) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using Resource within kubernetes types, where deepcopy-gen is used.
func (in *Resource) DeepCopyInto(out *Resource) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Resource. Required by controller-gen.
func (in *Resource) DeepCopy() *Resource {
	if in == nil {
		return nil
	}
	out := new(Resource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new Resource. Required by controller-gen.
func (in *Resource) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using Status within kubernetes types, where deepcopy-gen is used.
func (in *Status) DeepCopyInto(out *Status) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Status. Required by controller-gen.
func (in *Status) DeepCopy() *Status {
	if in == nil {
		return nil
	}
	out := new(Status)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new Status. Required by controller-gen.
func (in *Status) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using Condition within kubernetes types, where deepcopy-gen is used.
func (in *Condition) DeepCopyInto(out *Condition) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Condition. Required by controller-gen.
func (in *Condition) DeepCopy() *Condition {
	if in == nil {
		return nil
	}
	out := new(Condition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new Condition. Required by controller-gen.
func (in *Condition) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using Reference within kubernetes types, where deepcopy-gen is used.
func (in *Reference) DeepCopyInto(out *Reference) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Reference. Required by controller-gen.
func (in *Reference) DeepCopy() *Reference {
	if in == nil {
		return nil
	}
	out := new(Reference)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new Reference. Required by controller-gen.
func (in *Reference) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using Tombstone within kubernetes types, where deepcopy-gen is used.
func (in *Tombstone) DeepCopyInto(out *Tombstone) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Tombstone. Required by controller-gen.
func (in *Tombstone) DeepCopy() *Tombstone {
	if in == nil {
		return nil
	}
	out := new(Tombstone)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new Tombstone. Required by controller-gen.
func (in *Tombstone) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using ReadRequest within kubernetes types, where deepcopy-gen is used.
func (in *ReadRequest) DeepCopyInto(out *ReadRequest) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ReadRequest. Required by controller-gen.
func (in *ReadRequest) DeepCopy() *ReadRequest {
	if in == nil {
		return nil
	}
	out := new(ReadRequest)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new ReadRequest. Required by controller-gen.
func (in *ReadRequest) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using ReadResponse within kubernetes types, where deepcopy-gen is used.
func (in *ReadResponse) DeepCopyInto(out *ReadResponse) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ReadResponse. Required by controller-gen.
func (in *ReadResponse) DeepCopy() *ReadResponse {
	if in == nil {
		return nil
	}
	out := new(ReadResponse)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new ReadResponse. Required by controller-gen.
func (in *ReadResponse) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using ListRequest within kubernetes types, where deepcopy-gen is used.
func (in *ListRequest) DeepCopyInto(out *ListRequest) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ListRequest. Required by controller-gen.
func (in *ListRequest) DeepCopy() *ListRequest {
	if in == nil {
		return nil
	}
	out := new(ListRequest)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new ListRequest. Required by controller-gen.
func (in *ListRequest) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using ListResponse within kubernetes types, where deepcopy-gen is used.
func (in *ListResponse) DeepCopyInto(out *ListResponse) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ListResponse. Required by controller-gen.
func (in *ListResponse) DeepCopy() *ListResponse {
	if in == nil {
		return nil
	}
	out := new(ListResponse)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new ListResponse. Required by controller-gen.
func (in *ListResponse) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using ListByOwnerRequest within kubernetes types, where deepcopy-gen is used.
func (in *ListByOwnerRequest) DeepCopyInto(out *ListByOwnerRequest) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ListByOwnerRequest. Required by controller-gen.
func (in *ListByOwnerRequest) DeepCopy() *ListByOwnerRequest {
	if in == nil {
		return nil
	}
	out := new(ListByOwnerRequest)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new ListByOwnerRequest. Required by controller-gen.
func (in *ListByOwnerRequest) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using ListByOwnerResponse within kubernetes types, where deepcopy-gen is used.
func (in *ListByOwnerResponse) DeepCopyInto(out *ListByOwnerResponse) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ListByOwnerResponse. Required by controller-gen.
func (in *ListByOwnerResponse) DeepCopy() *ListByOwnerResponse {
	if in == nil {
		return nil
	}
	out := new(ListByOwnerResponse)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new ListByOwnerResponse. Required by controller-gen.
func (in *ListByOwnerResponse) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using WriteRequest within kubernetes types, where deepcopy-gen is used.
func (in *WriteRequest) DeepCopyInto(out *WriteRequest) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WriteRequest. Required by controller-gen.
func (in *WriteRequest) DeepCopy() *WriteRequest {
	if in == nil {
		return nil
	}
	out := new(WriteRequest)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new WriteRequest. Required by controller-gen.
func (in *WriteRequest) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using WriteResponse within kubernetes types, where deepcopy-gen is used.
func (in *WriteResponse) DeepCopyInto(out *WriteResponse) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WriteResponse. Required by controller-gen.
func (in *WriteResponse) DeepCopy() *WriteResponse {
	if in == nil {
		return nil
	}
	out := new(WriteResponse)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new WriteResponse. Required by controller-gen.
func (in *WriteResponse) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using WriteStatusRequest within kubernetes types, where deepcopy-gen is used.
func (in *WriteStatusRequest) DeepCopyInto(out *WriteStatusRequest) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WriteStatusRequest. Required by controller-gen.
func (in *WriteStatusRequest) DeepCopy() *WriteStatusRequest {
	if in == nil {
		return nil
	}
	out := new(WriteStatusRequest)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new WriteStatusRequest. Required by controller-gen.
func (in *WriteStatusRequest) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using WriteStatusResponse within kubernetes types, where deepcopy-gen is used.
func (in *WriteStatusResponse) DeepCopyInto(out *WriteStatusResponse) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WriteStatusResponse. Required by controller-gen.
func (in *WriteStatusResponse) DeepCopy() *WriteStatusResponse {
	if in == nil {
		return nil
	}
	out := new(WriteStatusResponse)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new WriteStatusResponse. Required by controller-gen.
func (in *WriteStatusResponse) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using DeleteRequest within kubernetes types, where deepcopy-gen is used.
func (in *DeleteRequest) DeepCopyInto(out *DeleteRequest) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DeleteRequest. Required by controller-gen.
func (in *DeleteRequest) DeepCopy() *DeleteRequest {
	if in == nil {
		return nil
	}
	out := new(DeleteRequest)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new DeleteRequest. Required by controller-gen.
func (in *DeleteRequest) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using DeleteResponse within kubernetes types, where deepcopy-gen is used.
func (in *DeleteResponse) DeepCopyInto(out *DeleteResponse) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DeleteResponse. Required by controller-gen.
func (in *DeleteResponse) DeepCopy() *DeleteResponse {
	if in == nil {
		return nil
	}
	out := new(DeleteResponse)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new DeleteResponse. Required by controller-gen.
func (in *DeleteResponse) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using WatchListRequest within kubernetes types, where deepcopy-gen is used.
func (in *WatchListRequest) DeepCopyInto(out *WatchListRequest) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WatchListRequest. Required by controller-gen.
func (in *WatchListRequest) DeepCopy() *WatchListRequest {
	if in == nil {
		return nil
	}
	out := new(WatchListRequest)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new WatchListRequest. Required by controller-gen.
func (in *WatchListRequest) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using WatchEvent within kubernetes types, where deepcopy-gen is used.
func (in *WatchEvent) DeepCopyInto(out *WatchEvent) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WatchEvent. Required by controller-gen.
func (in *WatchEvent) DeepCopy() *WatchEvent {
	if in == nil {
		return nil
	}
	out := new(WatchEvent)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new WatchEvent. Required by controller-gen.
func (in *WatchEvent) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}
