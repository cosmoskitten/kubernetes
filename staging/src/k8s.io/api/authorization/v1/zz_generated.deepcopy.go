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

package v1

import (
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	conversion "k8s.io/apimachinery/pkg/conversion"
	runtime "k8s.io/apimachinery/pkg/runtime"
	reflect "reflect"
)

func init() {
	SchemeBuilder.Register(RegisterDeepCopies)
}

// RegisterDeepCopies adds deep-copy functions to the given scheme. Public
// to allow building arbitrary schemes.
func RegisterDeepCopies(scheme *runtime.Scheme) error {
	return scheme.AddGeneratedDeepCopyFuncs(
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_v1_LocalSubjectAccessReview, InType: reflect.TypeOf(&LocalSubjectAccessReview{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_v1_NonResourceAttributes, InType: reflect.TypeOf(&NonResourceAttributes{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_v1_NonResourceRule, InType: reflect.TypeOf(&NonResourceRule{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_v1_ResourceAttributes, InType: reflect.TypeOf(&ResourceAttributes{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_v1_ResourceRule, InType: reflect.TypeOf(&ResourceRule{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_v1_SelfSubjectAccessReview, InType: reflect.TypeOf(&SelfSubjectAccessReview{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_v1_SelfSubjectAccessReviewSpec, InType: reflect.TypeOf(&SelfSubjectAccessReviewSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_v1_SelfSubjectRulesReview, InType: reflect.TypeOf(&SelfSubjectRulesReview{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_v1_SubjectAccessReview, InType: reflect.TypeOf(&SubjectAccessReview{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_v1_SubjectAccessReviewSpec, InType: reflect.TypeOf(&SubjectAccessReviewSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_v1_SubjectAccessReviewStatus, InType: reflect.TypeOf(&SubjectAccessReviewStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: DeepCopy_v1_SubjectRulesReviewStatus, InType: reflect.TypeOf(&SubjectRulesReviewStatus{})},
	)
}

// DeepCopy_v1_LocalSubjectAccessReview is an autogenerated deepcopy function.
func DeepCopy_v1_LocalSubjectAccessReview(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*LocalSubjectAccessReview)
		out := out.(*LocalSubjectAccessReview)
		*out = *in
		if newVal, err := c.DeepCopy(&in.ObjectMeta); err != nil {
			return err
		} else {
			out.ObjectMeta = *newVal.(*meta_v1.ObjectMeta)
		}
		if err := DeepCopy_v1_SubjectAccessReviewSpec(&in.Spec, &out.Spec, c); err != nil {
			return err
		}
		return nil
	}
}

// DeepCopy_v1_NonResourceAttributes is an autogenerated deepcopy function.
func DeepCopy_v1_NonResourceAttributes(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*NonResourceAttributes)
		out := out.(*NonResourceAttributes)
		*out = *in
		return nil
	}
}

// DeepCopy_v1_NonResourceRule is an autogenerated deepcopy function.
func DeepCopy_v1_NonResourceRule(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*NonResourceRule)
		out := out.(*NonResourceRule)
		*out = *in
		if in.Verbs != nil {
			in, out := &in.Verbs, &out.Verbs
			*out = make([]string, len(*in))
			copy(*out, *in)
		}
		if in.NonResourceURLs != nil {
			in, out := &in.NonResourceURLs, &out.NonResourceURLs
			*out = make([]string, len(*in))
			copy(*out, *in)
		}
		return nil
	}
}

// DeepCopy_v1_ResourceAttributes is an autogenerated deepcopy function.
func DeepCopy_v1_ResourceAttributes(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*ResourceAttributes)
		out := out.(*ResourceAttributes)
		*out = *in
		return nil
	}
}

// DeepCopy_v1_ResourceRule is an autogenerated deepcopy function.
func DeepCopy_v1_ResourceRule(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*ResourceRule)
		out := out.(*ResourceRule)
		*out = *in
		if in.Verbs != nil {
			in, out := &in.Verbs, &out.Verbs
			*out = make([]string, len(*in))
			copy(*out, *in)
		}
		if in.APIGroups != nil {
			in, out := &in.APIGroups, &out.APIGroups
			*out = make([]string, len(*in))
			copy(*out, *in)
		}
		if in.Resources != nil {
			in, out := &in.Resources, &out.Resources
			*out = make([]string, len(*in))
			copy(*out, *in)
		}
		if in.ResourceNames != nil {
			in, out := &in.ResourceNames, &out.ResourceNames
			*out = make([]string, len(*in))
			copy(*out, *in)
		}
		return nil
	}
}

// DeepCopy_v1_SelfSubjectAccessReview is an autogenerated deepcopy function.
func DeepCopy_v1_SelfSubjectAccessReview(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*SelfSubjectAccessReview)
		out := out.(*SelfSubjectAccessReview)
		*out = *in
		if newVal, err := c.DeepCopy(&in.ObjectMeta); err != nil {
			return err
		} else {
			out.ObjectMeta = *newVal.(*meta_v1.ObjectMeta)
		}
		if err := DeepCopy_v1_SelfSubjectAccessReviewSpec(&in.Spec, &out.Spec, c); err != nil {
			return err
		}
		return nil
	}
}

// DeepCopy_v1_SelfSubjectAccessReviewSpec is an autogenerated deepcopy function.
func DeepCopy_v1_SelfSubjectAccessReviewSpec(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*SelfSubjectAccessReviewSpec)
		out := out.(*SelfSubjectAccessReviewSpec)
		*out = *in
		if in.ResourceAttributes != nil {
			in, out := &in.ResourceAttributes, &out.ResourceAttributes
			*out = new(ResourceAttributes)
			**out = **in
		}
		if in.NonResourceAttributes != nil {
			in, out := &in.NonResourceAttributes, &out.NonResourceAttributes
			*out = new(NonResourceAttributes)
			**out = **in
		}
		return nil
	}
}

// DeepCopy_v1_SelfSubjectRulesReview is an autogenerated deepcopy function.
func DeepCopy_v1_SelfSubjectRulesReview(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*SelfSubjectRulesReview)
		out := out.(*SelfSubjectRulesReview)
		*out = *in
		if err := DeepCopy_v1_SubjectRulesReviewStatus(&in.Status, &out.Status, c); err != nil {
			return err
		}
		return nil
	}
}

// DeepCopy_v1_SubjectAccessReview is an autogenerated deepcopy function.
func DeepCopy_v1_SubjectAccessReview(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*SubjectAccessReview)
		out := out.(*SubjectAccessReview)
		*out = *in
		if newVal, err := c.DeepCopy(&in.ObjectMeta); err != nil {
			return err
		} else {
			out.ObjectMeta = *newVal.(*meta_v1.ObjectMeta)
		}
		if err := DeepCopy_v1_SubjectAccessReviewSpec(&in.Spec, &out.Spec, c); err != nil {
			return err
		}
		return nil
	}
}

// DeepCopy_v1_SubjectAccessReviewSpec is an autogenerated deepcopy function.
func DeepCopy_v1_SubjectAccessReviewSpec(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*SubjectAccessReviewSpec)
		out := out.(*SubjectAccessReviewSpec)
		*out = *in
		if in.ResourceAttributes != nil {
			in, out := &in.ResourceAttributes, &out.ResourceAttributes
			*out = new(ResourceAttributes)
			**out = **in
		}
		if in.NonResourceAttributes != nil {
			in, out := &in.NonResourceAttributes, &out.NonResourceAttributes
			*out = new(NonResourceAttributes)
			**out = **in
		}
		if in.Groups != nil {
			in, out := &in.Groups, &out.Groups
			*out = make([]string, len(*in))
			copy(*out, *in)
		}
		if in.Extra != nil {
			in, out := &in.Extra, &out.Extra
			*out = make(map[string]ExtraValue)
			for key, val := range *in {
				if newVal, err := c.DeepCopy(&val); err != nil {
					return err
				} else {
					(*out)[key] = *newVal.(*ExtraValue)
				}
			}
		}
		return nil
	}
}

// DeepCopy_v1_SubjectAccessReviewStatus is an autogenerated deepcopy function.
func DeepCopy_v1_SubjectAccessReviewStatus(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*SubjectAccessReviewStatus)
		out := out.(*SubjectAccessReviewStatus)
		*out = *in
		return nil
	}
}

// DeepCopy_v1_SubjectRulesReviewStatus is an autogenerated deepcopy function.
func DeepCopy_v1_SubjectRulesReviewStatus(in interface{}, out interface{}, c *conversion.Cloner) error {
	{
		in := in.(*SubjectRulesReviewStatus)
		out := out.(*SubjectRulesReviewStatus)
		*out = *in
		if in.ResourceRules != nil {
			in, out := &in.ResourceRules, &out.ResourceRules
			*out = make([]ResourceRule, len(*in))
			for i := range *in {
				if err := DeepCopy_v1_ResourceRule(&(*in)[i], &(*out)[i], c); err != nil {
					return err
				}
			}
		}
		if in.NonResourceRules != nil {
			in, out := &in.NonResourceRules, &out.NonResourceRules
			*out = make([]NonResourceRule, len(*in))
			for i := range *in {
				if err := DeepCopy_v1_NonResourceRule(&(*in)[i], &(*out)[i], c); err != nil {
					return err
				}
			}
		}
		return nil
	}
}
