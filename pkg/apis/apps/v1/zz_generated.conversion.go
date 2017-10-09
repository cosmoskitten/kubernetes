// +build !ignore_autogenerated

/*
Copyright 2017 The Kubernetes Authors.

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

// This file was autogenerated by conversion-gen. Do not edit it manually!

package v1

import (
	v1 "k8s.io/api/apps/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	conversion "k8s.io/apimachinery/pkg/conversion"
	runtime "k8s.io/apimachinery/pkg/runtime"
	api_v1 "k8s.io/kubernetes/pkg/apis/core/v1"
	extensions "k8s.io/kubernetes/pkg/apis/extensions"
	unsafe "unsafe"
)

func init() {
	localSchemeBuilder.Register(RegisterConversions)
}

// RegisterConversions adds conversion functions to the given scheme.
// Public to allow building arbitrary schemes.
func RegisterConversions(scheme *runtime.Scheme) error {
	return scheme.AddGeneratedConversionFuncs(
		Convert_v1_DaemonSet_To_extensions_DaemonSet,
		Convert_extensions_DaemonSet_To_v1_DaemonSet,
		Convert_v1_DaemonSetList_To_extensions_DaemonSetList,
		Convert_extensions_DaemonSetList_To_v1_DaemonSetList,
		Convert_v1_DaemonSetSpec_To_extensions_DaemonSetSpec,
		Convert_extensions_DaemonSetSpec_To_v1_DaemonSetSpec,
		Convert_v1_DaemonSetStatus_To_extensions_DaemonSetStatus,
		Convert_extensions_DaemonSetStatus_To_v1_DaemonSetStatus,
		Convert_v1_DaemonSetUpdateStrategy_To_extensions_DaemonSetUpdateStrategy,
		Convert_extensions_DaemonSetUpdateStrategy_To_v1_DaemonSetUpdateStrategy,
		Convert_v1_RollingUpdateDaemonSet_To_extensions_RollingUpdateDaemonSet,
		Convert_extensions_RollingUpdateDaemonSet_To_v1_RollingUpdateDaemonSet,
	)
}

func autoConvert_v1_DaemonSet_To_extensions_DaemonSet(in *v1.DaemonSet, out *extensions.DaemonSet, s conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	if err := Convert_v1_DaemonSetSpec_To_extensions_DaemonSetSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_v1_DaemonSetStatus_To_extensions_DaemonSetStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func autoConvert_extensions_DaemonSet_To_v1_DaemonSet(in *extensions.DaemonSet, out *v1.DaemonSet, s conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	if err := Convert_extensions_DaemonSetSpec_To_v1_DaemonSetSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_extensions_DaemonSetStatus_To_v1_DaemonSetStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func autoConvert_v1_DaemonSetList_To_extensions_DaemonSetList(in *v1.DaemonSetList, out *extensions.DaemonSetList, s conversion.Scope) error {
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]extensions.DaemonSet, len(*in))
		for i := range *in {
			if err := Convert_v1_DaemonSet_To_extensions_DaemonSet(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

// Convert_v1_DaemonSetList_To_extensions_DaemonSetList is an autogenerated conversion function.
func Convert_v1_DaemonSetList_To_extensions_DaemonSetList(in *v1.DaemonSetList, out *extensions.DaemonSetList, s conversion.Scope) error {
	return autoConvert_v1_DaemonSetList_To_extensions_DaemonSetList(in, out, s)
}

func autoConvert_extensions_DaemonSetList_To_v1_DaemonSetList(in *extensions.DaemonSetList, out *v1.DaemonSetList, s conversion.Scope) error {
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]v1.DaemonSet, len(*in))
		for i := range *in {
			if err := Convert_extensions_DaemonSet_To_v1_DaemonSet(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

// Convert_extensions_DaemonSetList_To_v1_DaemonSetList is an autogenerated conversion function.
func Convert_extensions_DaemonSetList_To_v1_DaemonSetList(in *extensions.DaemonSetList, out *v1.DaemonSetList, s conversion.Scope) error {
	return autoConvert_extensions_DaemonSetList_To_v1_DaemonSetList(in, out, s)
}

func autoConvert_v1_DaemonSetSpec_To_extensions_DaemonSetSpec(in *v1.DaemonSetSpec, out *extensions.DaemonSetSpec, s conversion.Scope) error {
	out.Selector = (*meta_v1.LabelSelector)(unsafe.Pointer(in.Selector))
	if err := api_v1.Convert_v1_PodTemplateSpec_To_api_PodTemplateSpec(&in.Template, &out.Template, s); err != nil {
		return err
	}
	if err := Convert_v1_DaemonSetUpdateStrategy_To_extensions_DaemonSetUpdateStrategy(&in.UpdateStrategy, &out.UpdateStrategy, s); err != nil {
		return err
	}
	out.MinReadySeconds = in.MinReadySeconds
	out.RevisionHistoryLimit = (*int32)(unsafe.Pointer(in.RevisionHistoryLimit))
	return nil
}

func autoConvert_extensions_DaemonSetSpec_To_v1_DaemonSetSpec(in *extensions.DaemonSetSpec, out *v1.DaemonSetSpec, s conversion.Scope) error {
	out.Selector = (*meta_v1.LabelSelector)(unsafe.Pointer(in.Selector))
	if err := api_v1.Convert_api_PodTemplateSpec_To_v1_PodTemplateSpec(&in.Template, &out.Template, s); err != nil {
		return err
	}
	if err := Convert_extensions_DaemonSetUpdateStrategy_To_v1_DaemonSetUpdateStrategy(&in.UpdateStrategy, &out.UpdateStrategy, s); err != nil {
		return err
	}
	out.MinReadySeconds = in.MinReadySeconds
	// WARNING: in.TemplateGeneration requires manual conversion: does not exist in peer-type
	out.RevisionHistoryLimit = (*int32)(unsafe.Pointer(in.RevisionHistoryLimit))
	return nil
}

func autoConvert_v1_DaemonSetStatus_To_extensions_DaemonSetStatus(in *v1.DaemonSetStatus, out *extensions.DaemonSetStatus, s conversion.Scope) error {
	out.CurrentNumberScheduled = in.CurrentNumberScheduled
	out.NumberMisscheduled = in.NumberMisscheduled
	out.DesiredNumberScheduled = in.DesiredNumberScheduled
	out.NumberReady = in.NumberReady
	out.ObservedGeneration = in.ObservedGeneration
	out.UpdatedNumberScheduled = in.UpdatedNumberScheduled
	out.NumberAvailable = in.NumberAvailable
	out.NumberUnavailable = in.NumberUnavailable
	out.CollisionCount = (*int32)(unsafe.Pointer(in.CollisionCount))
	return nil
}

// Convert_v1_DaemonSetStatus_To_extensions_DaemonSetStatus is an autogenerated conversion function.
func Convert_v1_DaemonSetStatus_To_extensions_DaemonSetStatus(in *v1.DaemonSetStatus, out *extensions.DaemonSetStatus, s conversion.Scope) error {
	return autoConvert_v1_DaemonSetStatus_To_extensions_DaemonSetStatus(in, out, s)
}

func autoConvert_extensions_DaemonSetStatus_To_v1_DaemonSetStatus(in *extensions.DaemonSetStatus, out *v1.DaemonSetStatus, s conversion.Scope) error {
	out.CurrentNumberScheduled = in.CurrentNumberScheduled
	out.NumberMisscheduled = in.NumberMisscheduled
	out.DesiredNumberScheduled = in.DesiredNumberScheduled
	out.NumberReady = in.NumberReady
	out.ObservedGeneration = in.ObservedGeneration
	out.UpdatedNumberScheduled = in.UpdatedNumberScheduled
	out.NumberAvailable = in.NumberAvailable
	out.NumberUnavailable = in.NumberUnavailable
	out.CollisionCount = (*int32)(unsafe.Pointer(in.CollisionCount))
	return nil
}

// Convert_extensions_DaemonSetStatus_To_v1_DaemonSetStatus is an autogenerated conversion function.
func Convert_extensions_DaemonSetStatus_To_v1_DaemonSetStatus(in *extensions.DaemonSetStatus, out *v1.DaemonSetStatus, s conversion.Scope) error {
	return autoConvert_extensions_DaemonSetStatus_To_v1_DaemonSetStatus(in, out, s)
}

func autoConvert_v1_DaemonSetUpdateStrategy_To_extensions_DaemonSetUpdateStrategy(in *v1.DaemonSetUpdateStrategy, out *extensions.DaemonSetUpdateStrategy, s conversion.Scope) error {
	out.Type = extensions.DaemonSetUpdateStrategyType(in.Type)
	if in.RollingUpdate != nil {
		in, out := &in.RollingUpdate, &out.RollingUpdate
		*out = new(extensions.RollingUpdateDaemonSet)
		if err := Convert_v1_RollingUpdateDaemonSet_To_extensions_RollingUpdateDaemonSet(*in, *out, s); err != nil {
			return err
		}
	} else {
		out.RollingUpdate = nil
	}
	return nil
}

func autoConvert_extensions_DaemonSetUpdateStrategy_To_v1_DaemonSetUpdateStrategy(in *extensions.DaemonSetUpdateStrategy, out *v1.DaemonSetUpdateStrategy, s conversion.Scope) error {
	out.Type = v1.DaemonSetUpdateStrategyType(in.Type)
	if in.RollingUpdate != nil {
		in, out := &in.RollingUpdate, &out.RollingUpdate
		*out = new(v1.RollingUpdateDaemonSet)
		if err := Convert_extensions_RollingUpdateDaemonSet_To_v1_RollingUpdateDaemonSet(*in, *out, s); err != nil {
			return err
		}
	} else {
		out.RollingUpdate = nil
	}
	return nil
}

func autoConvert_v1_RollingUpdateDaemonSet_To_extensions_RollingUpdateDaemonSet(in *v1.RollingUpdateDaemonSet, out *extensions.RollingUpdateDaemonSet, s conversion.Scope) error {
	// WARNING: in.MaxUnavailable requires manual conversion: inconvertible types (*k8s.io/apimachinery/pkg/util/intstr.IntOrString vs k8s.io/apimachinery/pkg/util/intstr.IntOrString)
	return nil
}

func autoConvert_extensions_RollingUpdateDaemonSet_To_v1_RollingUpdateDaemonSet(in *extensions.RollingUpdateDaemonSet, out *v1.RollingUpdateDaemonSet, s conversion.Scope) error {
	// WARNING: in.MaxUnavailable requires manual conversion: inconvertible types (k8s.io/apimachinery/pkg/util/intstr.IntOrString vs *k8s.io/apimachinery/pkg/util/intstr.IntOrString)
	return nil
}
