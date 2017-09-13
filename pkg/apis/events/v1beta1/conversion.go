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

package v1beta1

import (
	v1beta1 "k8s.io/api/events/v1beta1"
	conversion "k8s.io/apimachinery/pkg/conversion"
	api "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"
)

func Convert_v1beta1_Event_To_api_Event(in *v1beta1.Event, out *api.Event, s conversion.Scope) error {
	if err := autoConvert_v1beta1_Event_To_api_Event(in, out, s); err != nil {
		return err
	}
	if err := v1.Convert_v1_ObjectReference_To_api_ObjectReference(&in.Regarding, &out.InvolvedObject, s); err != nil {
		return err
	}
	if err := v1.Convert_v1_EventSource_To_api_EventSource(&in.DeprecatedSource, &out.Source, s); err != nil {
		return err
	}
	out.Message = in.Note
	out.FirstTimestamp = in.DeprecatedFirstTimestamp
	out.LastTimestamp = in.DeprecatedLastTimestamp
	out.Count = in.DeprecatedCount
	return nil
}

func Convert_api_Event_To_v1beta1_Event(in *api.Event, out *v1beta1.Event, s conversion.Scope) error {
	if err := autoConvert_api_Event_To_v1beta1_Event(in, out, s); err != nil {
		return err
	}
	if err := v1.Convert_api_ObjectReference_To_v1_ObjectReference(&in.InvolvedObject, &out.Regarding, s); err != nil {
		return err
	}
	if err := v1.Convert_api_EventSource_To_v1_EventSource(&in.Source, &out.DeprecatedSource, s); err != nil {
		return err
	}
	out.Note = in.Message
	out.DeprecatedFirstTimestamp = in.FirstTimestamp
	out.DeprecatedLastTimestamp = in.LastTimestamp
	out.DeprecatedCount = in.Count
	return nil
}
