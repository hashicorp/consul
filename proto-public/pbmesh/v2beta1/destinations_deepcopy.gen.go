// Code generated by protoc-gen-deepcopy. DO NOT EDIT.
package meshv2beta1

import (
	proto "google.golang.org/protobuf/proto"
)

// DeepCopyInto supports using Destinations within kubernetes types, where deepcopy-gen is used.
func (in *Destinations) DeepCopyInto(out *Destinations) {
	p := proto.Clone(in).(*Destinations)
	*out = *p
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Destinations. Required by controller-gen.
func (in *Destinations) DeepCopy() *Destinations {
	if in == nil {
		return nil
	}
	out := new(Destinations)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new Destinations. Required by controller-gen.
func (in *Destinations) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using Destination within kubernetes types, where deepcopy-gen is used.
func (in *Destination) DeepCopyInto(out *Destination) {
	p := proto.Clone(in).(*Destination)
	*out = *p
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Destination. Required by controller-gen.
func (in *Destination) DeepCopy() *Destination {
	if in == nil {
		return nil
	}
	out := new(Destination)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new Destination. Required by controller-gen.
func (in *Destination) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using IPPortAddress within kubernetes types, where deepcopy-gen is used.
func (in *IPPortAddress) DeepCopyInto(out *IPPortAddress) {
	p := proto.Clone(in).(*IPPortAddress)
	*out = *p
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IPPortAddress. Required by controller-gen.
func (in *IPPortAddress) DeepCopy() *IPPortAddress {
	if in == nil {
		return nil
	}
	out := new(IPPortAddress)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new IPPortAddress. Required by controller-gen.
func (in *IPPortAddress) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using UnixSocketAddress within kubernetes types, where deepcopy-gen is used.
func (in *UnixSocketAddress) DeepCopyInto(out *UnixSocketAddress) {
	p := proto.Clone(in).(*UnixSocketAddress)
	*out = *p
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new UnixSocketAddress. Required by controller-gen.
func (in *UnixSocketAddress) DeepCopy() *UnixSocketAddress {
	if in == nil {
		return nil
	}
	out := new(UnixSocketAddress)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new UnixSocketAddress. Required by controller-gen.
func (in *UnixSocketAddress) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using PreparedQueryDestination within kubernetes types, where deepcopy-gen is used.
func (in *PreparedQueryDestination) DeepCopyInto(out *PreparedQueryDestination) {
	p := proto.Clone(in).(*PreparedQueryDestination)
	*out = *p
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PreparedQueryDestination. Required by controller-gen.
func (in *PreparedQueryDestination) DeepCopy() *PreparedQueryDestination {
	if in == nil {
		return nil
	}
	out := new(PreparedQueryDestination)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new PreparedQueryDestination. Required by controller-gen.
func (in *PreparedQueryDestination) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}
