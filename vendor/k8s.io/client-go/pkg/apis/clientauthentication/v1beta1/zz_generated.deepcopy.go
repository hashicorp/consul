// +build !ignore_autogenerated

/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by deepcopy-gen. DO NOT EDIT.

package v1beta1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExecCredential) DeepCopyInto(out *ExecCredential) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.Spec = in.Spec
	if in.Status != nil {
		in, out := &in.Status, &out.Status
		if *in == nil {
			*out = nil
		} else {
			*out = new(ExecCredentialStatus)
			(*in).DeepCopyInto(*out)
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExecCredential.
func (in *ExecCredential) DeepCopy() *ExecCredential {
	if in == nil {
		return nil
	}
	out := new(ExecCredential)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ExecCredential) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExecCredentialSpec) DeepCopyInto(out *ExecCredentialSpec) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExecCredentialSpec.
func (in *ExecCredentialSpec) DeepCopy() *ExecCredentialSpec {
	if in == nil {
		return nil
	}
	out := new(ExecCredentialSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExecCredentialStatus) DeepCopyInto(out *ExecCredentialStatus) {
	*out = *in
	if in.ExpirationTimestamp != nil {
		in, out := &in.ExpirationTimestamp, &out.ExpirationTimestamp
		if *in == nil {
			*out = nil
		} else {
			*out = (*in).DeepCopy()
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExecCredentialStatus.
func (in *ExecCredentialStatus) DeepCopy() *ExecCredentialStatus {
	if in == nil {
		return nil
	}
	out := new(ExecCredentialStatus)
	in.DeepCopyInto(out)
	return out
}
