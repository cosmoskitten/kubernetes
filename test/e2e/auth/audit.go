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

package auth

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	apiv1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apiextensions-apiserver/test/integration/testserver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/apis/audit/v1beta1"
	"k8s.io/kubernetes/test/e2e/framework"
	imageutils "k8s.io/kubernetes/test/utils/image"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	watchTimeout = 1
	user         = "kubecfg"
)

var _ = SIGDescribe("Advanced Audit [Feature:Audit]", func() {
	f := framework.NewDefaultFramework("audit")

	It("should audit API calls", func() {
		namespace := f.Namespace.Name

		pod := &apiv1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "audit-pod",
			},
			Spec: apiv1.PodSpec{
				Containers: []apiv1.Container{{
					Name:  "pause",
					Image: framework.GetPauseImageName(f.ClientSet),
				}},
			},
		}

		podLabels := map[string]string{"name": "audit-deployment-pod"}
		d := framework.NewDeployment("audit-deployment", int32(1), podLabels, "redis", imageutils.GetE2EImage(imageutils.Redis), extensions.RecreateDeploymentStrategyType)

		secret := &apiv1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "audit-secret",
			},
			Data: map[string][]byte{
				"top-secret": []byte("foo-bar"),
			},
		}

		configMap := &apiv1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: "audit-configmap",
			},
			Data: map[string]string{
				"map-key": "map-value",
			},
		}

		watchOptions := metav1.ListOptions{TimeoutSeconds: intToPointer(watchTimeout)}

		f.PodClient().CreateSync(pod)
		_, err := f.PodClient().Get(pod.Name, metav1.GetOptions{})
		framework.ExpectNoError(err, "failed to get audit-pod")
		_, err = f.PodClient().Watch(watchOptions)
		framework.ExpectNoError(err, "failed to create watch for pods")
		_, err = f.PodClient().List(metav1.ListOptions{})
		framework.ExpectNoError(err, "failed to list pods")
		f.PodClient().DeleteSync(pod.Name, &metav1.DeleteOptions{}, framework.DefaultPodDeletionTimeout)

		_, err = f.ClientSet.Extensions().Deployments(f.Namespace.Name).Create(d)
		framework.ExpectNoError(err, "failed to create audit-deployment")
		_, err = f.ClientSet.Extensions().Deployments(f.Namespace.Name).Get(d.Name, metav1.GetOptions{})
		framework.ExpectNoError(err, "failed to get audit-deployment")
		_, err = f.ClientSet.Extensions().Deployments(f.Namespace.Name).Watch(watchOptions)
		framework.ExpectNoError(err, "failed to create watch for deployments")
		_, err = f.ClientSet.Extensions().Deployments(f.Namespace.Name).List(metav1.ListOptions{})
		framework.ExpectNoError(err, "failed to create list deployments")
		err = f.ClientSet.Extensions().Deployments(f.Namespace.Name).Delete("audit-deployment", &metav1.DeleteOptions{})
		framework.ExpectNoError(err, "failed to delete deployments")

		_, err = f.ClientSet.Core().Secrets(f.Namespace.Name).Create(secret)
		framework.ExpectNoError(err, "failed to create audit-secret")
		_, err = f.ClientSet.Core().Secrets(f.Namespace.Name).Get(secret.Name, metav1.GetOptions{})
		framework.ExpectNoError(err, "failed to get audit-secret")
		_, err = f.ClientSet.Core().Secrets(f.Namespace.Name).List(metav1.ListOptions{})
		framework.ExpectNoError(err, "failed to list secrets")
		_, err = f.ClientSet.Core().Secrets(f.Namespace.Name).Watch(watchOptions)
		framework.ExpectNoError(err, "failed to create watch for secrets")
		err = f.ClientSet.Core().Secrets(f.Namespace.Name).Delete(secret.Name, &metav1.DeleteOptions{})
		framework.ExpectNoError(err, "failed to delete audit-secret")

		_, err = f.ClientSet.Core().ConfigMaps(f.Namespace.Name).Create(configMap)
		framework.ExpectNoError(err, "failed to create audit-configmap")
		_, err = f.ClientSet.Core().ConfigMaps(f.Namespace.Name).Get(configMap.Name, metav1.GetOptions{})
		framework.ExpectNoError(err, "failed to get audit-configmap")
		_, err = f.ClientSet.Core().ConfigMaps(f.Namespace.Name).List(metav1.ListOptions{})
		framework.ExpectNoError(err, "failed to list config maps")
		_, err = f.ClientSet.Core().ConfigMaps(f.Namespace.Name).Watch(watchOptions)
		framework.ExpectNoError(err, "failed to create watch for config maps")
		err = f.ClientSet.Core().ConfigMaps(f.Namespace.Name).Delete(configMap.Name, &metav1.DeleteOptions{})
		framework.ExpectNoError(err, "failed to delete audit-configmap")

		config, err := framework.LoadConfig()
		if err != nil {
			framework.Failf("failed to load config: %v", err)
		}
		apiExtensionClient, err := clientset.NewForConfig(config)
		if err != nil {
			framework.Failf("failed to initialize apiExtensionClient: %v", err)
		}
		crd := testserver.NewRandomNameCustomResourceDefinition(apiextensionsv1beta1.ClusterScoped)
		_, err = testserver.CreateNewCustomResourceDefinition(crd, apiExtensionClient, f.ClientPool)
		testserver.DeleteCustomResourceDefinition(crd, apiExtensionClient)

		expectedEvents := []auditEvent{}
		expectedEvents = append(expectedEvents, commonExpectedEvents("pods", namespace, "create", pod.Name)...)
		expectedEvents = append(expectedEvents, commonExpectedEvents("pods", namespace, "delete", pod.Name)...)
		expectedEvents = append(expectedEvents, commonExpectedEvents("pods", namespace, "get", pod.Name)...)
		expectedEvents = append(expectedEvents, commonExpectedEvents("pods", namespace, "list", pod.Name)...)

		expectedEvents = append(expectedEvents, commonExpectedEvents("deployments", namespace, "create", d.Name)...)
		expectedEvents = append(expectedEvents, commonExpectedEvents("deployments", namespace, "delete", d.Name)...)
		expectedEvents = append(expectedEvents, commonExpectedEvents("deployments", namespace, "get", d.Name)...)
		expectedEvents = append(expectedEvents, commonExpectedEvents("deployments", namespace, "list", d.Name)...)

		expectedEvents = append(expectedEvents, commonExpectedEvents("secrets", namespace, "create", secret.Name)...)
		expectedEvents = append(expectedEvents, commonExpectedEvents("secrets", namespace, "delete", secret.Name)...)
		expectedEvents = append(expectedEvents, commonExpectedEvents("secrets", namespace, "get", secret.Name)...)
		expectedEvents = append(expectedEvents, commonExpectedEvents("secrets", namespace, "list", secret.Name)...)

		expectedEvents = append(expectedEvents, commonExpectedEvents("configmaps", namespace, "create", configMap.Name)...)
		expectedEvents = append(expectedEvents, commonExpectedEvents("configmaps", namespace, "delete", configMap.Name)...)
		expectedEvents = append(expectedEvents, commonExpectedEvents("configmaps", namespace, "get", configMap.Name)...)
		expectedEvents = append(expectedEvents, commonExpectedEvents("configmaps", namespace, "list", configMap.Name)...)

		expectedEvents = append(expectedEvents, watchExpectedEvents("pods", namespace)...)
		expectedEvents = append(expectedEvents, watchExpectedEvents("deployments", namespace)...)
		expectedEvents = append(expectedEvents, watchExpectedEvents("secrets", namespace)...)
		expectedEvents = append(expectedEvents, watchExpectedEvents("configmaps", namespace)...)

		expectedEvents = append(expectedEvents, customResourceEvents("create", *crd)...)
		expectedEvents = append(expectedEvents, customResourceEvents("delete", *crd)...)

		// Sleep for watches to timeout.
		time.Sleep(3 * watchTimeout * time.Second)

		expectAuditLines(f, expectedEvents)
	})
})

func intToPointer(num int64) *int64 {
	return &num
}

type auditEvent struct {
	level          v1beta1.Level
	stage          v1beta1.Stage
	requestURI     string
	verb           string
	code           int32
	user           string
	resource       string
	namespace      string
	requestObject  bool
	responseObject bool
}

// Search the audit log for the expected audit lines.
func expectAuditLines(f *framework.Framework, expected []auditEvent) {
	expectations := map[auditEvent]bool{}
	for _, event := range expected {
		expectations[event] = false
	}

	// Fetch the log stream.
	stream, err := f.ClientSet.Core().RESTClient().Get().AbsPath("/logs/kube-apiserver-audit.log").Stream()
	framework.ExpectNoError(err, "could not read audit log")
	defer stream.Close()

	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		line := scanner.Text()
		event, err := parseAuditLine(line)
		framework.ExpectNoError(err)

		// If the event was expected, mark it as found.
		if _, found := expectations[event]; found {
			expectations[event] = true
		}
	}
	framework.ExpectNoError(scanner.Err(), "error reading audit log")

	for event, found := range expectations {
		Expect(found).To(BeTrue(), "Event %#v not found!", event)
	}
}

func parseAuditLine(line string) (auditEvent, error) {
	var e v1beta1.Event
	if err := json.Unmarshal([]byte(line), &e); err != nil {
		return auditEvent{}, err
	}
	event := auditEvent{
		level:      e.Level,
		stage:      e.Stage,
		requestURI: e.RequestURI,
		verb:       e.Verb,
		user:       e.User.Username,
	}
	if e.ObjectRef != nil {
		event.namespace = e.ObjectRef.Namespace
		event.resource = e.ObjectRef.Resource
	}
	if e.ResponseStatus != nil {
		event.code = e.ResponseStatus.Code
	}
	if e.ResponseObject != nil {
		event.responseObject = true
	}
	if e.RequestObject != nil {
		event.requestObject = true
	}
	return event, nil
}

func customResourceEvents(method string, crd apiextensionsv1beta1.CustomResourceDefinition) []auditEvent {
	ar := strings.SplitN(crd.Name, ".", 2)
	name := ar[0]
	namespace := ar[1]
	apiEvent := auditEvent{
		level:          v1beta1.LevelRequestResponse,
		stage:          v1beta1.StageResponseComplete,
		requestURI:     "/apis/apiextensions.k8s.io/v1beta1/customresourcedefinitions",
		verb:           method,
		code:           code(method),
		user:           user,
		resource:       "customresourcedefinitions",
		requestObject:  true,
		responseObject: true,
	}
	objectEvent := auditEvent{
		level:          v1beta1.LevelMetadata,
		stage:          v1beta1.StageResponseComplete,
		requestURI:     fmt.Sprintf("/apis/%s/v1beta1/%s", namespace, name),
		verb:           method,
		code:           code(method),
		user:           user,
		resource:       name,
		requestObject:  false,
		responseObject: false,
	}
	if method == "delete" {
		apiEvent.requestObject = false
		apiEvent.requestURI = fmt.Sprintf("/apis/apiextensions.k8s.io/v1beta1/customresourcedefinitions/%s", crd.Name)
		objectEvent.requestURI = fmt.Sprintf("/apis/%s/v1beta1/%s/setup-instance", namespace, name)
	}
	return []auditEvent{apiEvent, objectEvent}
}

func commonExpectedEvents(resource string, namespace string, method string, resourceName string) []auditEvent {
	return []auditEvent{
		{
			level:          level(resource, method),
			stage:          v1beta1.StageResponseComplete,
			requestURI:     uriString(resource, namespace, method, resourceName),
			verb:           method,
			code:           code(method),
			user:           user,
			resource:       resource,
			namespace:      namespace,
			requestObject:  containsRequestBody(resource, method),
			responseObject: containsRequestBody(resource, method),
		},
	}
}

func containsRequestBody(resource string, method string) bool {
	if (resource == "pods" || resource == "deployments") && (method == "create" || method == "delete") {
		return true
	} else {
		return false
	}
}

func uriString(resource string, namespace string, method string, resourceName string) string {
	if method == "create" || method == "list" {
		return fmt.Sprintf("%s/namespaces/%s/%s", api(resource), namespace, resource)
	} else {
		return fmt.Sprintf("%s/namespaces/%s/%s/%s", api(resource), namespace, resource, resourceName)
	}
}

func code(method string) int32 {
	if method == "create" {
		return 201
	} else {
		return 200
	}
}

func level(resource string, method string) v1beta1.Level {
	if resource == "secrets" || resource == "configmaps" {
		return v1beta1.LevelMetadata
	} else if method == "delete" || method == "create" {
		return v1beta1.LevelRequestResponse
	} else {
		return v1beta1.LevelRequest
	}
}

func api(resource string) string {
	if resource == "deployments" {
		return "/apis/extensions/v1beta1"
	} else {
		return "/api/v1"
	}
}

func watchExpectedEvents(resource string, namespace string) []auditEvent {
	responseStarted := auditEvent{
		level:          level(resource, ""),
		stage:          v1beta1.StageResponseStarted,
		requestURI:     fmt.Sprintf("%s/namespaces/%s/%s?timeoutSeconds=%d&watch=true", api(resource), namespace, resource, watchTimeout),
		verb:           "watch",
		code:           200,
		user:           user,
		resource:       resource,
		namespace:      namespace,
		requestObject:  false,
		responseObject: false,
	}
	responseComplete := responseStarted
	responseComplete.stage = v1beta1.StageResponseComplete
	return []auditEvent{
		responseStarted,
		responseComplete,
	}
}
