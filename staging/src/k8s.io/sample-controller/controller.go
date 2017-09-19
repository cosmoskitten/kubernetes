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

package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"
	apps "k8s.io/api/apps/v1beta1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	appsinformers "k8s.io/client-go/informers/apps/v1beta1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"

	"k8s.io/apimachinery/pkg/runtime/schema"
	example "k8s.io/sample-controller/pkg/apis/example/v1alpha1"
	"k8s.io/sample-controller/pkg/client/clientset/versioned"
	intinformers "k8s.io/sample-controller/pkg/client/informers/externalversions"
	exampleinformers "k8s.io/sample-controller/pkg/client/informers/externalversions/example/v1alpha1"
	"reflect"
)

type Controller struct {
	clientset        kubernetes.Interface
	exampleclientset versioned.Interface

	informerFactory        informers.SharedInformerFactory
	exampleInformerFactory intinformers.SharedInformerFactory

	deploymentsInformer appsinformers.DeploymentInformer
	nginxInformer       exampleinformers.NGINXInformer

	workqueue workqueue.RateLimitingInterface
}

func NewController() (*Controller, error) {
	cfg, err := kubeConfig()

	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(cfg)

	if err != nil {
		return nil, err
	}

	exampleclientset, err := versioned.NewForConfig(cfg)

	if err != nil {
		return nil, err
	}

	// Create informer factories used to share the same instances of informers
	// between multiple control loops
	informerFactory := informers.NewSharedInformerFactory(clientset, time.Second*30)
	exampleInformerFactory := intinformers.NewSharedInformerFactory(exampleclientset, time.Second*30)

	return &Controller{
		clientset:              clientset,
		exampleclientset:       exampleclientset,
		informerFactory:        informerFactory,
		exampleInformerFactory: exampleInformerFactory,
		deploymentsInformer:    informerFactory.Apps().V1beta1().Deployments(),
		nginxInformer:          exampleInformerFactory.Example().V1alpha1().NGINXs(),
		workqueue:              workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "NGINXs"),
	}, nil
}

func (c *Controller) Run(stopCh <-chan struct{}) error {
	glog.Info("Setting up event handlers")
	c.nginxInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueueNGINX,
		UpdateFunc: func(old, new interface{}) {
			if !reflect.DeepEqual(old, new) {
				c.enqueueNGINX(new)
			}
		},
	})
	c.deploymentsInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.handleObject,
		UpdateFunc: func(old, new interface{}) {
			if !reflect.DeepEqual(old, new) {
				c.handleObject(new)
			}
		},
		DeleteFunc: c.handleObject,
	})

	glog.Info("Starting informer factories")
	go c.informerFactory.Start(stopCh)
	go c.exampleInformerFactory.Start(stopCh)

	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.nginxInformer.Informer().HasSynced, c.deploymentsInformer.Informer().HasSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	glog.Info("Starting workers...")
	// We use a WaitGroup here so that upon initial shutdown signal, we wait
	// for the items that are currently being processed to finish processing
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.worker()
		}()
	}

	glog.Info("Started workers")
	<-stopCh
	glog.Info("Shutting down workers")
	c.workqueue.ShutDown()
	wg.Wait()
	glog.Info("Workers shutdown")

	return nil
}

func (c *Controller) worker() {
	for {
		obj, shutdown := c.workqueue.Get()

		if shutdown {
			return
		}

		err := func(obj interface{}) error {
			defer c.workqueue.Done(obj)
			var key string
			var ok bool
			if key, ok = obj.(string); !ok {
				c.workqueue.Forget(obj)
				runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
				return nil
			}
			if err := c.syncHandler(key); err != nil {
				return fmt.Errorf("error syncing '%s': %s", key, err.Error())
			}
			c.workqueue.Forget(obj)
			return nil
		}(obj)

		if err != nil {
			runtime.HandleError(err)
			continue
		}
	}
}

func (c *Controller) syncHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	nginx, err := c.nginxInformer.Lister().NGINXs(namespace).Get(name)

	if err != nil {
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("nginx '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	deploymentName := nginx.Spec.DeploymentName
	if deploymentName == "" {
		// we don't return an error here so that the item doesn't get requeued
		// until it is next modified
		runtime.HandleError(fmt.Errorf("%s: deployment name must be specified", key))
		return nil
	}

	deployment, err := c.deploymentsInformer.Lister().Deployments(nginx.Namespace).Get(deploymentName)
	if errors.IsNotFound(err) {
		deployment, err = c.clientset.AppsV1beta1().Deployments(nginx.Namespace).Create(newDeployment(nginx))
	}

	if err != nil {
		return err
	}

	return c.updateNGINXStatus(nginx, deployment)
}

func (c *Controller) updateNGINXStatus(nginx *example.NGINX, deployment *apps.Deployment) error {
	update := nginx.DeepCopy()
	update.Status.AvailableReplicas = deployment.Status.AvailableReplicas
	_, err := c.exampleclientset.ExampleV1alpha1().NGINXs(nginx.Namespace).Update(update)
	return err
}

func (c *Controller) enqueueNGINX(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

func (c *Controller) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		glog.Errorf("error decoding object, invalid type")
		return
	}
	glog.V(4).Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		nginx, err := c.nginxInformer.Lister().NGINXs(object.GetNamespace()).Get(ownerRef.Name)

		if err != nil {
			glog.V(4).Infof("ignoring orphaned object '%s' of nginx '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		c.enqueueNGINX(nginx)
		return
	}
}

func newDeployment(nginx *example.NGINX) *apps.Deployment {
	return &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nginx.Spec.DeploymentName,
			Namespace: nginx.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(nginx, schema.GroupVersionKind{
					Group:   example.SchemeGroupVersion.Group,
					Version: example.SchemeGroupVersion.Version,
					Kind:    "NGINX",
				}),
			},
		},
		Spec: apps.DeploymentSpec{
			Replicas: nginx.Spec.Replicas,
			Template: core.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":        "nginx",
						"controller": nginx.Name,
					},
				},
				Spec: core.PodSpec{
					Containers: []core.Container{
						{
							Name:  "nginx",
							Image: "nginx:latest",
						},
					},
				},
			},
		},
	}
}

func kubeConfig() (*rest.Config, error) {
	apiCfg, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()

	if err != nil {
		return nil, fmt.Errorf("error loading cluster config: %s", err.Error())
	}

	cfg, err := clientcmd.NewDefaultClientConfig(*apiCfg, &clientcmd.ConfigOverrides{}).ClientConfig()

	if err != nil {
		return nil, fmt.Errorf("error loading cluster client config: %s", err.Error())
	}

	return cfg, nil
}
