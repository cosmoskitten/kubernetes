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

package monitoring

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	CustomMetricName = "foo-metric"
	UnusedMetricName = "unused-metric"
	MetricValue1     = int64(448)
	MetricValue2     = int64(446)
)

func SDExporterDeployment(name string, replicas int32, metricValue int64) *extensions.Deployment {
	return &extensions.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: extensions.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"name": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"name": name,
					},
				},
				Spec: sdExporterPodSpec(CustomMetricName, metricValue),
			},
			Replicas: &replicas,
		},
	}
}

func sdExporterPodSpec(metricName string, metricValue int64) corev1.PodSpec {
	return corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:            "sd-exporter",
				Image:           "gcr.io/google-containers/sd-dummy-exporter:v0.1.0",
				ImagePullPolicy: corev1.PullPolicy("Always"),
				Command:         []string{"/sd_dummy_exporter", "--pod-id=$(POD_ID)", "--metric-name=" + metricName, fmt.Sprintf("--metric-value=%v", metricValue)},
				Env: []corev1.EnvVar{
					{
						Name: "POD_ID",
						ValueFrom: &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: "metadata.uid",
							},
						},
					},
				},
				Ports: []corev1.ContainerPort{{ContainerPort: 80}},
			},
		},
	}
}

func SDExporterPod(podName, podLabel, metricName string, metricValue int64) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: "default",
			Labels: map[string]string{
				"name": podLabel,
			},
		},
		Spec: sdExporterPodSpec(metricName, metricValue),
	}
}
