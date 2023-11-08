// Code generated by protoc-gen-deepcopy. DO NOT EDIT.
package authv2beta1

import (
	proto "google.golang.org/protobuf/proto"
)

// DeepCopyInto supports using TrafficPermissions within kubernetes types, where deepcopy-gen is used.
func (in *TrafficPermissions) DeepCopyInto(out *TrafficPermissions) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TrafficPermissions. Required by controller-gen.
func (in *TrafficPermissions) DeepCopy() *TrafficPermissions {
	if in == nil {
		return nil
	}
	out := new(TrafficPermissions)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new TrafficPermissions. Required by controller-gen.
func (in *TrafficPermissions) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using NamespaceTrafficPermissions within kubernetes types, where deepcopy-gen is used.
func (in *NamespaceTrafficPermissions) DeepCopyInto(out *NamespaceTrafficPermissions) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NamespaceTrafficPermissions. Required by controller-gen.
func (in *NamespaceTrafficPermissions) DeepCopy() *NamespaceTrafficPermissions {
	if in == nil {
		return nil
	}
	out := new(NamespaceTrafficPermissions)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new NamespaceTrafficPermissions. Required by controller-gen.
func (in *NamespaceTrafficPermissions) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using PartitionTrafficPermissions within kubernetes types, where deepcopy-gen is used.
func (in *PartitionTrafficPermissions) DeepCopyInto(out *PartitionTrafficPermissions) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PartitionTrafficPermissions. Required by controller-gen.
func (in *PartitionTrafficPermissions) DeepCopy() *PartitionTrafficPermissions {
	if in == nil {
		return nil
	}
	out := new(PartitionTrafficPermissions)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new PartitionTrafficPermissions. Required by controller-gen.
func (in *PartitionTrafficPermissions) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using Destination within kubernetes types, where deepcopy-gen is used.
func (in *Destination) DeepCopyInto(out *Destination) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
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

// DeepCopyInto supports using Permission within kubernetes types, where deepcopy-gen is used.
func (in *Permission) DeepCopyInto(out *Permission) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Permission. Required by controller-gen.
func (in *Permission) DeepCopy() *Permission {
	if in == nil {
		return nil
	}
	out := new(Permission)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new Permission. Required by controller-gen.
func (in *Permission) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using Source within kubernetes types, where deepcopy-gen is used.
func (in *Source) DeepCopyInto(out *Source) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Source. Required by controller-gen.
func (in *Source) DeepCopy() *Source {
	if in == nil {
		return nil
	}
	out := new(Source)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new Source. Required by controller-gen.
func (in *Source) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using ExcludeSource within kubernetes types, where deepcopy-gen is used.
func (in *ExcludeSource) DeepCopyInto(out *ExcludeSource) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExcludeSource. Required by controller-gen.
func (in *ExcludeSource) DeepCopy() *ExcludeSource {
	if in == nil {
		return nil
	}
	out := new(ExcludeSource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new ExcludeSource. Required by controller-gen.
func (in *ExcludeSource) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using DestinationRule within kubernetes types, where deepcopy-gen is used.
func (in *DestinationRule) DeepCopyInto(out *DestinationRule) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DestinationRule. Required by controller-gen.
func (in *DestinationRule) DeepCopy() *DestinationRule {
	if in == nil {
		return nil
	}
	out := new(DestinationRule)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new DestinationRule. Required by controller-gen.
func (in *DestinationRule) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using ExcludePermissionRule within kubernetes types, where deepcopy-gen is used.
func (in *ExcludePermissionRule) DeepCopyInto(out *ExcludePermissionRule) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExcludePermissionRule. Required by controller-gen.
func (in *ExcludePermissionRule) DeepCopy() *ExcludePermissionRule {
	if in == nil {
		return nil
	}
	out := new(ExcludePermissionRule)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new ExcludePermissionRule. Required by controller-gen.
func (in *ExcludePermissionRule) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using DestinationRuleHeader within kubernetes types, where deepcopy-gen is used.
func (in *DestinationRuleHeader) DeepCopyInto(out *DestinationRuleHeader) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DestinationRuleHeader. Required by controller-gen.
func (in *DestinationRuleHeader) DeepCopy() *DestinationRuleHeader {
	if in == nil {
		return nil
	}
	out := new(DestinationRuleHeader)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new DestinationRuleHeader. Required by controller-gen.
func (in *DestinationRuleHeader) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}
