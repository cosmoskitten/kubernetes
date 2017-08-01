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

// This file was autogenerated by deepcopy-gen. Do not edit it manually!

package apiextensions

import (
	conversion "k8s.io/apimachinery/pkg/conversion"
	runtime "k8s.io/apimachinery/pkg/runtime"
	reflect "reflect"
)

// Deprecated: register deep-copy functions.
func init() {
	SchemeBuilder.Register(RegisterDeepCopies)
}

// Deprecated: RegisterDeepCopies adds deep-copy functions to the given scheme. Public
// to allow building arbitrary schemes.
func RegisterDeepCopies(scheme *runtime.Scheme) error {
	return scheme.AddGeneratedDeepCopyFuncs(
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*CustomResourceDefinition).DeepCopyInto(out.(*CustomResourceDefinition))
			return nil
		}, InType: reflect.TypeOf(&CustomResourceDefinition{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*CustomResourceDefinitionCondition).DeepCopyInto(out.(*CustomResourceDefinitionCondition))
			return nil
		}, InType: reflect.TypeOf(&CustomResourceDefinitionCondition{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*CustomResourceDefinitionList).DeepCopyInto(out.(*CustomResourceDefinitionList))
			return nil
		}, InType: reflect.TypeOf(&CustomResourceDefinitionList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*CustomResourceDefinitionNames).DeepCopyInto(out.(*CustomResourceDefinitionNames))
			return nil
		}, InType: reflect.TypeOf(&CustomResourceDefinitionNames{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*CustomResourceDefinitionSpec).DeepCopyInto(out.(*CustomResourceDefinitionSpec))
			return nil
		}, InType: reflect.TypeOf(&CustomResourceDefinitionSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*CustomResourceDefinitionStatus).DeepCopyInto(out.(*CustomResourceDefinitionStatus))
			return nil
		}, InType: reflect.TypeOf(&CustomResourceDefinitionStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*CustomResourceValidation).DeepCopyInto(out.(*CustomResourceValidation))
			return nil
		}, InType: reflect.TypeOf(&CustomResourceValidation{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ExternalDocumentation).DeepCopyInto(out.(*ExternalDocumentation))
			return nil
		}, InType: reflect.TypeOf(&ExternalDocumentation{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*JSONSchemaProps).DeepCopyInto(out.(*JSONSchemaProps))
			return nil
		}, InType: reflect.TypeOf(&JSONSchemaProps{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*JSONSchemaPropsOrArray).DeepCopyInto(out.(*JSONSchemaPropsOrArray))
			return nil
		}, InType: reflect.TypeOf(&JSONSchemaPropsOrArray{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*JSONSchemaPropsOrBool).DeepCopyInto(out.(*JSONSchemaPropsOrBool))
			return nil
		}, InType: reflect.TypeOf(&JSONSchemaPropsOrBool{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*JSONSchemaPropsOrStringArray).DeepCopyInto(out.(*JSONSchemaPropsOrStringArray))
			return nil
		}, InType: reflect.TypeOf(&JSONSchemaPropsOrStringArray{})},
	)
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CustomResourceDefinition) DeepCopyInto(out *CustomResourceDefinition) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, creating a new CustomResourceDefinition.
func (x *CustomResourceDefinition) DeepCopy() *CustomResourceDefinition {
	if x == nil {
		return nil
	}
	out := new(CustomResourceDefinition)
	x.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (x *CustomResourceDefinition) DeepCopyObject() runtime.Object {
	if c := x.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CustomResourceDefinitionCondition) DeepCopyInto(out *CustomResourceDefinitionCondition) {
	*out = *in
	in.LastTransitionTime.DeepCopyInto(&out.LastTransitionTime)
	return
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, creating a new CustomResourceDefinitionCondition.
func (x *CustomResourceDefinitionCondition) DeepCopy() *CustomResourceDefinitionCondition {
	if x == nil {
		return nil
	}
	out := new(CustomResourceDefinitionCondition)
	x.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CustomResourceDefinitionList) DeepCopyInto(out *CustomResourceDefinitionList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]CustomResourceDefinition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, creating a new CustomResourceDefinitionList.
func (x *CustomResourceDefinitionList) DeepCopy() *CustomResourceDefinitionList {
	if x == nil {
		return nil
	}
	out := new(CustomResourceDefinitionList)
	x.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (x *CustomResourceDefinitionList) DeepCopyObject() runtime.Object {
	if c := x.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CustomResourceDefinitionNames) DeepCopyInto(out *CustomResourceDefinitionNames) {
	*out = *in
	if in.ShortNames != nil {
		in, out := &in.ShortNames, &out.ShortNames
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, creating a new CustomResourceDefinitionNames.
func (x *CustomResourceDefinitionNames) DeepCopy() *CustomResourceDefinitionNames {
	if x == nil {
		return nil
	}
	out := new(CustomResourceDefinitionNames)
	x.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CustomResourceDefinitionSpec) DeepCopyInto(out *CustomResourceDefinitionSpec) {
	*out = *in
	in.Names.DeepCopyInto(&out.Names)
	in.Validation.DeepCopyInto(&out.Validation)
	return
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, creating a new CustomResourceDefinitionSpec.
func (x *CustomResourceDefinitionSpec) DeepCopy() *CustomResourceDefinitionSpec {
	if x == nil {
		return nil
	}
	out := new(CustomResourceDefinitionSpec)
	x.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CustomResourceDefinitionStatus) DeepCopyInto(out *CustomResourceDefinitionStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]CustomResourceDefinitionCondition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	in.AcceptedNames.DeepCopyInto(&out.AcceptedNames)
	return
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, creating a new CustomResourceDefinitionStatus.
func (x *CustomResourceDefinitionStatus) DeepCopy() *CustomResourceDefinitionStatus {
	if x == nil {
		return nil
	}
	out := new(CustomResourceDefinitionStatus)
	x.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CustomResourceValidation) DeepCopyInto(out *CustomResourceValidation) {
	*out = *in
	if in.JSONSchema != nil {
		in, out := &in.JSONSchema, &out.JSONSchema
		if *in == nil {
			*out = nil
		} else {
			*out = new(JSONSchemaProps)
			(*in).DeepCopyInto(*out)
		}
	}
	return
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, creating a new CustomResourceValidation.
func (x *CustomResourceValidation) DeepCopy() *CustomResourceValidation {
	if x == nil {
		return nil
	}
	out := new(CustomResourceValidation)
	x.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExternalDocumentation) DeepCopyInto(out *ExternalDocumentation) {
	*out = *in
	return
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, creating a new ExternalDocumentation.
func (x *ExternalDocumentation) DeepCopy() *ExternalDocumentation {
	if x == nil {
		return nil
	}
	out := new(ExternalDocumentation)
	x.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *JSONSchemaProps) DeepCopyInto(out *JSONSchemaProps) {
	clone := in.DeepCopy()
	*out = *clone
	return
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *JSONSchemaPropsOrArray) DeepCopyInto(out *JSONSchemaPropsOrArray) {
	*out = *in
	if in.Schema != nil {
		in, out := &in.Schema, &out.Schema
		if *in == nil {
			*out = nil
		} else {
			*out = new(JSONSchemaProps)
			(*in).DeepCopyInto(*out)
		}
	}
	if in.JSONSchemas != nil {
		in, out := &in.JSONSchemas, &out.JSONSchemas
		*out = make([]JSONSchemaProps, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, creating a new JSONSchemaPropsOrArray.
func (x *JSONSchemaPropsOrArray) DeepCopy() *JSONSchemaPropsOrArray {
	if x == nil {
		return nil
	}
	out := new(JSONSchemaPropsOrArray)
	x.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *JSONSchemaPropsOrBool) DeepCopyInto(out *JSONSchemaPropsOrBool) {
	*out = *in
	if in.Schema != nil {
		in, out := &in.Schema, &out.Schema
		if *in == nil {
			*out = nil
		} else {
			*out = new(JSONSchemaProps)
			(*in).DeepCopyInto(*out)
		}
	}
	return
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, creating a new JSONSchemaPropsOrBool.
func (x *JSONSchemaPropsOrBool) DeepCopy() *JSONSchemaPropsOrBool {
	if x == nil {
		return nil
	}
	out := new(JSONSchemaPropsOrBool)
	x.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *JSONSchemaPropsOrStringArray) DeepCopyInto(out *JSONSchemaPropsOrStringArray) {
	*out = *in
	if in.Schema != nil {
		in, out := &in.Schema, &out.Schema
		if *in == nil {
			*out = nil
		} else {
			*out = new(JSONSchemaProps)
			(*in).DeepCopyInto(*out)
		}
	}
	if in.Property != nil {
		in, out := &in.Property, &out.Property
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, creating a new JSONSchemaPropsOrStringArray.
func (x *JSONSchemaPropsOrStringArray) DeepCopy() *JSONSchemaPropsOrStringArray {
	if x == nil {
		return nil
	}
	out := new(JSONSchemaPropsOrStringArray)
	x.DeepCopyInto(out)
	return out
}
