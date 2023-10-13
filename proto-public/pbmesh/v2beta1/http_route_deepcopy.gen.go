// Code generated by protoc-gen-deepcopy. DO NOT EDIT.
package meshv2beta1

import (
	proto "google.golang.org/protobuf/proto"
)

// DeepCopyInto supports using HTTPRoute within kubernetes types, where deepcopy-gen is used.
func (in *HTTPRoute) DeepCopyInto(out *HTTPRoute) {
	p := proto.Clone(in).(*HTTPRoute)
	*out = *p
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HTTPRoute. Required by controller-gen.
func (in *HTTPRoute) DeepCopy() *HTTPRoute {
	if in == nil {
		return nil
	}
	out := new(HTTPRoute)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new HTTPRoute. Required by controller-gen.
func (in *HTTPRoute) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using HTTPRouteRule within kubernetes types, where deepcopy-gen is used.
func (in *HTTPRouteRule) DeepCopyInto(out *HTTPRouteRule) {
	p := proto.Clone(in).(*HTTPRouteRule)
	*out = *p
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HTTPRouteRule. Required by controller-gen.
func (in *HTTPRouteRule) DeepCopy() *HTTPRouteRule {
	if in == nil {
		return nil
	}
	out := new(HTTPRouteRule)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new HTTPRouteRule. Required by controller-gen.
func (in *HTTPRouteRule) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using HTTPRouteMatch within kubernetes types, where deepcopy-gen is used.
func (in *HTTPRouteMatch) DeepCopyInto(out *HTTPRouteMatch) {
	p := proto.Clone(in).(*HTTPRouteMatch)
	*out = *p
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HTTPRouteMatch. Required by controller-gen.
func (in *HTTPRouteMatch) DeepCopy() *HTTPRouteMatch {
	if in == nil {
		return nil
	}
	out := new(HTTPRouteMatch)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new HTTPRouteMatch. Required by controller-gen.
func (in *HTTPRouteMatch) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using HTTPPathMatch within kubernetes types, where deepcopy-gen is used.
func (in *HTTPPathMatch) DeepCopyInto(out *HTTPPathMatch) {
	p := proto.Clone(in).(*HTTPPathMatch)
	*out = *p
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HTTPPathMatch. Required by controller-gen.
func (in *HTTPPathMatch) DeepCopy() *HTTPPathMatch {
	if in == nil {
		return nil
	}
	out := new(HTTPPathMatch)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new HTTPPathMatch. Required by controller-gen.
func (in *HTTPPathMatch) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using HTTPHeaderMatch within kubernetes types, where deepcopy-gen is used.
func (in *HTTPHeaderMatch) DeepCopyInto(out *HTTPHeaderMatch) {
	p := proto.Clone(in).(*HTTPHeaderMatch)
	*out = *p
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HTTPHeaderMatch. Required by controller-gen.
func (in *HTTPHeaderMatch) DeepCopy() *HTTPHeaderMatch {
	if in == nil {
		return nil
	}
	out := new(HTTPHeaderMatch)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new HTTPHeaderMatch. Required by controller-gen.
func (in *HTTPHeaderMatch) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using HTTPQueryParamMatch within kubernetes types, where deepcopy-gen is used.
func (in *HTTPQueryParamMatch) DeepCopyInto(out *HTTPQueryParamMatch) {
	p := proto.Clone(in).(*HTTPQueryParamMatch)
	*out = *p
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HTTPQueryParamMatch. Required by controller-gen.
func (in *HTTPQueryParamMatch) DeepCopy() *HTTPQueryParamMatch {
	if in == nil {
		return nil
	}
	out := new(HTTPQueryParamMatch)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new HTTPQueryParamMatch. Required by controller-gen.
func (in *HTTPQueryParamMatch) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using HTTPRouteFilter within kubernetes types, where deepcopy-gen is used.
func (in *HTTPRouteFilter) DeepCopyInto(out *HTTPRouteFilter) {
	p := proto.Clone(in).(*HTTPRouteFilter)
	*out = *p
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HTTPRouteFilter. Required by controller-gen.
func (in *HTTPRouteFilter) DeepCopy() *HTTPRouteFilter {
	if in == nil {
		return nil
	}
	out := new(HTTPRouteFilter)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new HTTPRouteFilter. Required by controller-gen.
func (in *HTTPRouteFilter) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using HTTPHeaderFilter within kubernetes types, where deepcopy-gen is used.
func (in *HTTPHeaderFilter) DeepCopyInto(out *HTTPHeaderFilter) {
	p := proto.Clone(in).(*HTTPHeaderFilter)
	*out = *p
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HTTPHeaderFilter. Required by controller-gen.
func (in *HTTPHeaderFilter) DeepCopy() *HTTPHeaderFilter {
	if in == nil {
		return nil
	}
	out := new(HTTPHeaderFilter)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new HTTPHeaderFilter. Required by controller-gen.
func (in *HTTPHeaderFilter) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using HTTPHeader within kubernetes types, where deepcopy-gen is used.
func (in *HTTPHeader) DeepCopyInto(out *HTTPHeader) {
	p := proto.Clone(in).(*HTTPHeader)
	*out = *p
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HTTPHeader. Required by controller-gen.
func (in *HTTPHeader) DeepCopy() *HTTPHeader {
	if in == nil {
		return nil
	}
	out := new(HTTPHeader)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new HTTPHeader. Required by controller-gen.
func (in *HTTPHeader) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using HTTPURLRewriteFilter within kubernetes types, where deepcopy-gen is used.
func (in *HTTPURLRewriteFilter) DeepCopyInto(out *HTTPURLRewriteFilter) {
	p := proto.Clone(in).(*HTTPURLRewriteFilter)
	*out = *p
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HTTPURLRewriteFilter. Required by controller-gen.
func (in *HTTPURLRewriteFilter) DeepCopy() *HTTPURLRewriteFilter {
	if in == nil {
		return nil
	}
	out := new(HTTPURLRewriteFilter)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new HTTPURLRewriteFilter. Required by controller-gen.
func (in *HTTPURLRewriteFilter) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}

// DeepCopyInto supports using HTTPBackendRef within kubernetes types, where deepcopy-gen is used.
func (in *HTTPBackendRef) DeepCopyInto(out *HTTPBackendRef) {
	p := proto.Clone(in).(*HTTPBackendRef)
	*out = *p
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HTTPBackendRef. Required by controller-gen.
func (in *HTTPBackendRef) DeepCopy() *HTTPBackendRef {
	if in == nil {
		return nil
	}
	out := new(HTTPBackendRef)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInterface is an autogenerated deepcopy function, copying the receiver, creating a new HTTPBackendRef. Required by controller-gen.
func (in *HTTPBackendRef) DeepCopyInterface() interface{} {
	return in.DeepCopy()
}
