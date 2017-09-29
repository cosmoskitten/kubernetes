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
	"context"
	"time"

	"golang.org/x/oauth2/google"
	clientset "k8s.io/client-go/kubernetes"

	. "github.com/onsi/ginkgo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	instrumentation "k8s.io/kubernetes/test/e2e/instrumentation/common"

	gcm "google.golang.org/api/monitoring/v3"
	autoscaling "k8s.io/api/autoscaling/v2beta1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	stackdriverExporterDeployment = "stackdriver-exporter-deployment"
	dummyDeploymentName           = "dummy-deployment"
	stackdriverExporterPod        = "stackdriver-exporter-pod"
)

var _ = instrumentation.SIGDescribe("Stackdriver Monitoring", func() {
	BeforeEach(func() {
		framework.SkipUnlessProviderIs("gce", "gke")
	})

	f := framework.NewDefaultFramework("stackdriver-monitoring")
	var kubeClient clientset.Interface

	It("should autoscale with Custom Metrics from Stackdriver [Feature:StackdriverMonitoring]", func() {
		kubeClient = f.ClientSet
		testHPA(f, kubeClient)
	})
})

func testHPA(f *framework.Framework, kubeClient clientset.Interface) {
	projectId := framework.TestContext.CloudConfig.ProjectID

	ctx := context.Background()
	client, err := google.DefaultClient(ctx, gcm.CloudPlatformScope)

	// Hack for running tests locally, needed to authenticate in Stackdriver
	// If this is your use case, create application default credentials:
	// $ gcloud auth application-default login
	// and uncomment following lines:
	/*
		ts, err := google.DefaultTokenSource(oauth2.NoContext)
		framework.Logf("Couldn't get application default credentials, %v", err)
		if err != nil {
			framework.Failf("Error accessing application default credentials, %v", err)
		}
		client := oauth2.NewClient(oauth2.NoContext, ts)
	*/

	gcmService, err := gcm.New(client)
	if err != nil {
		framework.Failf("Failed to create gcm service, %v", err)
	}

	// Set up a cluster: create a custom metric and set up k8s-sd adapter
	err = createDescriptors(gcmService, projectId)
	if err != nil {
		framework.Failf("Failed to create metric descriptor: %v", err)
	}
	defer cleanupDescriptors(gcmService, projectId)

	err = createAdapter()
	if err != nil {
		framework.Failf("Failed to set up: %v", err)
	}
	defer cleanupAdapter()

	// Run application that exports the metric
	err = createDeploymentsToScale(f, kubeClient)
	if err != nil {
		framework.Failf("Failed to create stackdriver-exporter pod: %v", err)
	}
	defer cleanupDeploymentsToScale(f, kubeClient)

	// Autoscale the deployments
	err = createPodsHPA(f, kubeClient)
	if err != nil {
		framework.Failf("Failed to create 'Pods' HPA: %v", err)
	}
	err = createObjectHPA(f, kubeClient)
	if err != nil {
		framework.Failf("Failed to create 'Objects' HPA: %v", err)
	}

	// Wait a for HPA to scale down targets
	// TODO: wait for the deployments to reach the expected numbers of replicas instead of fixed
	//       amount of time
	time.Sleep(240 * time.Second)

	// Verify that the deployments were scaled down to the minimum value
	sdExporterDeployment, err := kubeClient.Extensions().Deployments(f.Namespace.ObjectMeta.Name).Get(stackdriverExporterDeployment, metav1.GetOptions{})
	if err != nil {
		framework.Failf("Failed to retrieve info about " + stackdriverExporterDeployment)
	}
	if *sdExporterDeployment.Spec.Replicas != 1 {
		framework.Failf("Unexpected number of replicas for %s. Expected 1, but received %v", stackdriverExporterDeployment, *sdExporterDeployment.Spec.Replicas)
	}
	dummyDeployment, err := kubeClient.Extensions().Deployments(f.Namespace.ObjectMeta.Name).Get(dummyDeploymentName, metav1.GetOptions{})
	if err != nil {
		framework.Failf("Failed to retrieve info about " + dummyDeploymentName)
	}
	if *sdExporterDeployment.Spec.Replicas != 1 {
		framework.Failf("Unexpected number of replicas for %s. Expected 1, but received %v", dummyDeploymentName, *dummyDeployment.Spec.Replicas)
	}
}

func createDeploymentsToScale(f *framework.Framework, cs clientset.Interface) error {
	_, err := cs.Extensions().Deployments(f.Namespace.ObjectMeta.Name).Create(StackdriverExporterDeployment(stackdriverExporterDeployment, 2, 100))
	if err != nil {
		return err
	}
	_, err = cs.Core().Pods(f.Namespace.ObjectMeta.Name).Create(StackdriverExporterPod(stackdriverExporterPod, stackdriverExporterPod, CustomMetricName, 100))
	if err != nil {
		return err
	}
	_, err = cs.Extensions().Deployments(f.Namespace.ObjectMeta.Name).Create(StackdriverExporterDeployment(dummyDeploymentName, 2, 100))
	return err
}

func cleanupDeploymentsToScale(f *framework.Framework, cs clientset.Interface) {
	_ = cs.Extensions().Deployments(f.Namespace.ObjectMeta.Name).Delete(stackdriverExporterDeployment, &metav1.DeleteOptions{})
	_ = cs.Core().Pods(f.Namespace.ObjectMeta.Name).Delete(stackdriverExporterPod, &metav1.DeleteOptions{})
	_ = cs.Extensions().Deployments(f.Namespace.ObjectMeta.Name).Delete(dummyDeploymentName, &metav1.DeleteOptions{})
}

func createPodsHPA(f *framework.Framework, cs clientset.Interface) error {
	var minReplicas int32 = 1
	_, err := cs.AutoscalingV2beta1().HorizontalPodAutoscalers(f.Namespace.ObjectMeta.Name).Create(&autoscaling.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "custom-metrics-pods-hpa",
			Namespace: f.Namespace.ObjectMeta.Name,
		},
		Spec: autoscaling.HorizontalPodAutoscalerSpec{
			Metrics: []autoscaling.MetricSpec{
				{
					Type: autoscaling.PodsMetricSourceType,
					Pods: &autoscaling.PodsMetricSource{
						MetricName:         CustomMetricName,
						TargetAverageValue: *resource.NewQuantity(200, resource.DecimalSI),
					},
				},
			},
			MaxReplicas: 3,
			MinReplicas: &minReplicas,
			ScaleTargetRef: autoscaling.CrossVersionObjectReference{
				APIVersion: "extensions/v1beta1",
				Kind:       "Deployment",
				Name:       stackdriverExporterDeployment,
			},
		},
	})
	return err
}

func createObjectHPA(f *framework.Framework, cs clientset.Interface) error {
	var minReplicas int32 = 1
	_, err := cs.AutoscalingV2beta1().HorizontalPodAutoscalers(f.Namespace.ObjectMeta.Name).Create(&autoscaling.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "custom-metrics-objects-hpa",
			Namespace: f.Namespace.ObjectMeta.Name,
		},
		Spec: autoscaling.HorizontalPodAutoscalerSpec{
			Metrics: []autoscaling.MetricSpec{
				{
					Type: autoscaling.ObjectMetricSourceType,
					Object: &autoscaling.ObjectMetricSource{
						MetricName: CustomMetricName,
						Target: autoscaling.CrossVersionObjectReference{
							Kind: "Pod",
							Name: stackdriverExporterPod,
						},
						TargetValue: *resource.NewQuantity(200, resource.DecimalSI),
					},
				},
			},
			MaxReplicas: 3,
			MinReplicas: &minReplicas,
			ScaleTargetRef: autoscaling.CrossVersionObjectReference{
				APIVersion: "extensions/v1beta1",
				Kind:       "Deployment",
				Name:       dummyDeploymentName,
			},
		},
	})
	return err
}
