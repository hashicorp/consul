// Code generated by protoc-gen-deepcopy. DO NOT EDIT.
package pbdataplane

import (
	proto "google.golang.org/protobuf/proto"
)

// DeepCopyInto supports using GetSupportedDataplaneFeaturesRequest within kubernetes types, where deepcopy-gen is used.
func (in *GetSupportedDataplaneFeaturesRequest) DeepCopyInto(out *GetSupportedDataplaneFeaturesRequest) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GetSupportedDataplaneFeaturesRequest. Required by controller-gen.
func (in *GetSupportedDataplaneFeaturesRequest) DeepCopy() *GetSupportedDataplaneFeaturesRequest {
	if in == nil {
		return nil
	}
	out := new(GetSupportedDataplaneFeaturesRequest)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new GetSupportedDataplaneFeaturesRequest. Required by controller-gen.
func (in *GetSupportedDataplaneFeaturesRequest) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using DataplaneFeatureSupport within kubernetes types, where deepcopy-gen is used.
func (in *DataplaneFeatureSupport) DeepCopyInto(out *DataplaneFeatureSupport) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DataplaneFeatureSupport. Required by controller-gen.
func (in *DataplaneFeatureSupport) DeepCopy() *DataplaneFeatureSupport {
	if in == nil {
		return nil
	}
	out := new(DataplaneFeatureSupport)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new DataplaneFeatureSupport. Required by controller-gen.
func (in *DataplaneFeatureSupport) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using GetSupportedDataplaneFeaturesResponse within kubernetes types, where deepcopy-gen is used.
func (in *GetSupportedDataplaneFeaturesResponse) DeepCopyInto(out *GetSupportedDataplaneFeaturesResponse) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GetSupportedDataplaneFeaturesResponse. Required by controller-gen.
func (in *GetSupportedDataplaneFeaturesResponse) DeepCopy() *GetSupportedDataplaneFeaturesResponse {
	if in == nil {
		return nil
	}
	out := new(GetSupportedDataplaneFeaturesResponse)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new GetSupportedDataplaneFeaturesResponse. Required by controller-gen.
func (in *GetSupportedDataplaneFeaturesResponse) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using GetEnvoyBootstrapParamsRequest within kubernetes types, where deepcopy-gen is used.
func (in *GetEnvoyBootstrapParamsRequest) DeepCopyInto(out *GetEnvoyBootstrapParamsRequest) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GetEnvoyBootstrapParamsRequest. Required by controller-gen.
func (in *GetEnvoyBootstrapParamsRequest) DeepCopy() *GetEnvoyBootstrapParamsRequest {
	if in == nil {
		return nil
	}
	out := new(GetEnvoyBootstrapParamsRequest)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new GetEnvoyBootstrapParamsRequest. Required by controller-gen.
func (in *GetEnvoyBootstrapParamsRequest) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using GetEnvoyBootstrapParamsResponse within kubernetes types, where deepcopy-gen is used.
func (in *GetEnvoyBootstrapParamsResponse) DeepCopyInto(out *GetEnvoyBootstrapParamsResponse) {
	proto.Reset(out)
	proto.Merge(out, proto.Clone(in))
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GetEnvoyBootstrapParamsResponse. Required by controller-gen.
func (in *GetEnvoyBootstrapParamsResponse) DeepCopy() *GetEnvoyBootstrapParamsResponse {
	if in == nil {
		return nil
	}
	out := new(GetEnvoyBootstrapParamsResponse)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new GetEnvoyBootstrapParamsResponse. Required by controller-gen.
func (in *GetEnvoyBootstrapParamsResponse) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}
