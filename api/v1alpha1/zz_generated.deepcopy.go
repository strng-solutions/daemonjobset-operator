// +build !ignore_autogenerated

/*
Copyright 2021.

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

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	"k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CronJobTemplateSpec) DeepCopyInto(out *CronJobTemplateSpec) {
	*out = *in
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CronJobTemplateSpec.
func (in *CronJobTemplateSpec) DeepCopy() *CronJobTemplateSpec {
	if in == nil {
		return nil
	}
	out := new(CronJobTemplateSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DaemonJobSet) DeepCopyInto(out *DaemonJobSet) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DaemonJobSet.
func (in *DaemonJobSet) DeepCopy() *DaemonJobSet {
	if in == nil {
		return nil
	}
	out := new(DaemonJobSet)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DaemonJobSet) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DaemonJobSetList) DeepCopyInto(out *DaemonJobSetList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]DaemonJobSet, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DaemonJobSetList.
func (in *DaemonJobSetList) DeepCopy() *DaemonJobSetList {
	if in == nil {
		return nil
	}
	out := new(DaemonJobSetList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DaemonJobSetList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DaemonJobSetPlacement) DeepCopyInto(out *DaemonJobSetPlacement) {
	*out = *in
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DaemonJobSetPlacement.
func (in *DaemonJobSetPlacement) DeepCopy() *DaemonJobSetPlacement {
	if in == nil {
		return nil
	}
	out := new(DaemonJobSetPlacement)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DaemonJobSetSpec) DeepCopyInto(out *DaemonJobSetSpec) {
	*out = *in
	if in.Suspend != nil {
		in, out := &in.Suspend, &out.Suspend
		*out = new(bool)
		**out = **in
	}
	in.Placement.DeepCopyInto(&out.Placement)
	in.CronJobTemplate.DeepCopyInto(&out.CronJobTemplate)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DaemonJobSetSpec.
func (in *DaemonJobSetSpec) DeepCopy() *DaemonJobSetSpec {
	if in == nil {
		return nil
	}
	out := new(DaemonJobSetSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DaemonJobSetStatus) DeepCopyInto(out *DaemonJobSetStatus) {
	*out = *in
	if in.CronJobs != nil {
		in, out := &in.CronJobs, &out.CronJobs
		*out = make([]v1.ObjectReference, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DaemonJobSetStatus.
func (in *DaemonJobSetStatus) DeepCopy() *DaemonJobSetStatus {
	if in == nil {
		return nil
	}
	out := new(DaemonJobSetStatus)
	in.DeepCopyInto(out)
	return out
}
