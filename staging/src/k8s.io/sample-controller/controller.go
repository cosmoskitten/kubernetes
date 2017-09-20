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
	"reflect"
	"sync"
	"time"

	"github.com/golang/glog"
	apps "k8s.io/api/apps/v1beta1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	appsinformers "k8s.io/client-go/informers/apps/v1beta1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	example "k8s.io/sample-controller/pkg/apis/example/v1alpha1"
	"k8s.io/sample-controller/pkg/client/clientset/versioned"
	intinformers "k8s.io/sample-controller/pkg/client/informers/externalversions"
	exampleinformers "k8s.io/sample-controller/pkg/client/informers/externalversions/example/v1alpha1"
)

type Controller struct {
	// clientset is a standard kubernetes clientset
	clientset kubernetes.Interface
	// exampleclientset is a clientset for our own API group
	exampleclientset versioned.Interface

	// informerFactory produces shared informers for kubernetes types
	informerFactory informers.SharedInformerFactory
	// exampleInformerFactory produces shared informers for our own API group
	exampleInformerFactory intinformers.SharedInformerFactory

	// deploymentsInformer is used to watch for changes to deployments, and for
	// it's underlying lister
	deploymentsInformer appsinformers.DeploymentInformer
	// fooInformer is used to watch for changes to Foos, and for it's
	// underlying lister
	fooInformer exampleinformers.FooInformer

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
}

// NewController returns a new controller for the given kubeconfig
func NewController(cfg *rest.Config) (*Controller, error) {
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
		fooInformer:            exampleInformerFactory.Example().V1alpha1().Foos(),
		workqueue:              workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Foos"),
	}, nil
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(stopCh <-chan struct{}) error {
	glog.Info("Setting up event handlers")
	// Set up an event handler for when Foo resources change
	c.fooInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueueFoo,
		UpdateFunc: func(old, new interface{}) {
			if !reflect.DeepEqual(old, new) {
				c.enqueueFoo(new)
			}
		},
	})
	// Set up an event handler for when Deployment resources change. This
	// handler will lookup the owner of the given Deployment, and if it is
	// owned by a Foo resource will enqueue that Foo resource for
	// processing. This way, we don't need to implement custom logic for
	// handling Deployment resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	c.deploymentsInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.handleObject,
		UpdateFunc: func(old, new interface{}) {
			if !reflect.DeepEqual(old, new) {
				c.handleObject(new)
			}
		},
		DeleteFunc: c.handleObject,
	})

	// Start the informer factories to begin populating the informer caches
	glog.Info("Starting informer factories")
	go c.informerFactory.Start(stopCh)
	go c.exampleInformerFactory.Start(stopCh)

	// Wait for the caches to be synced before starting workers
	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.fooInformer.Informer().HasSynced, c.deploymentsInformer.Informer().HasSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	glog.Info("Starting workers...")
	// We use a WaitGroup here so that upon initial shutdown signal, we wait
	// for the items that are currently being processed to finish processing
	var wg sync.WaitGroup
	// Launch two workers to process Foo resources
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

// worker is a long-running function that will continually read messages off
// workqueue and attempt to process them. It will run until the workqueue has
// been shutdown by a call to workqueue.ShutDown().
func (c *Controller) worker() {
	for {
		obj, shutdown := c.workqueue.Get()

		if shutdown {
			return
		}

		// We wrap this block in a func so we can defer c.workqueue.Done.
		err := func(obj interface{}) error {
			// We call Done here so the workqueue knows we have finished
			// processing this item. We also must remember to call Forget if we
			// do not want this work item being re-queued. For example, we do
			// not call Forget if a transient error occurs, instead the item is
			// put back on the workqueue and attempted again after a back-off
			// period.
			defer c.workqueue.Done(obj)
			var key string
			var ok bool
			// We expect strings to come off the workqueue. These are of the
			// form namespace/name. We do this as the delayed nature of the
			// workqueue means the items in the informer cache may actually be
			// more up to date that when the item was initially put onto the
			// workqueue.
			if key, ok = obj.(string); !ok {
				// As the item in the workqueue is actually invalid, we call
				// Forget here else we'd go into a loop of attempting to
				// process a work item that is invalid.
				c.workqueue.Forget(obj)
				runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
				return nil
			}
			// Run the syncHandler, passing it the namespace/name string of the
			// Foo resource to be synced.
			if err := c.syncHandler(key); err != nil {
				return fmt.Errorf("error syncing '%s': %s", key, err.Error())
			}
			// Finally, if no error occurs we Forget this item so it does not
			// get queued again until another change happens.
			c.workqueue.Forget(obj)
			return nil
		}(obj)

		if err != nil {
			runtime.HandleError(err)
			continue
		}
	}
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Foo resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the Foo resource with this namespace/name
	foo, err := c.fooInformer.Lister().Foos(namespace).Get(name)

	if err != nil {
		// The Foo resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("foo '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	deploymentName := foo.Spec.DeploymentName
	if deploymentName == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		runtime.HandleError(fmt.Errorf("%s: deployment name must be specified", key))
		return nil
	}

	// Get the deployment with the name specified in Foo.spec
	deployment, err := c.deploymentsInformer.Lister().Deployments(foo.Namespace).Get(deploymentName)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		deployment, err = c.clientset.AppsV1beta1().Deployments(foo.Namespace).Create(newDeployment(foo))
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// If this number of the replicas on the Foo resource is specified, and the
	// number does not equal the current desired replicas on the Deployment, we
	// should update the Deployment resource.
	if foo.Spec.Replicas != nil && *foo.Spec.Replicas != *deployment.Spec.Replicas {
		deployment, err = c.clientset.AppsV1beta1().Deployments(foo.Namespace).Update(newDeployment(foo))
	}

	// If an error occurs during Update, we'll requeue the item so we can
	// attempt processing again later. THis could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// Finally, we update the status block of the Foo resource to reflect the
	// current state of the world
	return c.updateFooStatus(foo, deployment)
}

func (c *Controller) updateFooStatus(foo *example.Foo, deployment *apps.Deployment) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	fooCopy := foo.DeepCopy()
	fooCopy.Status.AvailableReplicas = deployment.Status.AvailableReplicas
	// Until #38113 is merged, we must use Update instead of UpdateStatus to
	// update the Status block of the Foo resource. UpdateStatus will not
	// allow changes to the Spec of the resource, which is ideal for ensuring
	// nothing other than resource status has been updated.
	_, err := c.exampleclientset.ExampleV1alpha1().Foos(foo.Namespace).Update(fooCopy)
	return err
}

// enqueueFoo takes a Foo resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Foo.
func (c *Controller) enqueueFoo(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the Foo resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that Foo resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *Controller) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		glog.Errorf("error decoding object, invalid type")
		return
	}
	glog.V(4).Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a Foo, we should not do anything more
		// with it.
		if ownerRef.Kind != "Foo" {
			return
		}

		foo, err := c.fooInformer.Lister().Foos(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			glog.V(4).Infof("ignoring orphaned object '%s' of foo '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		c.enqueueFoo(foo)
		return
	}
}

// newDeployment creates a new Deployment for a Foo resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the Foo resource that 'owns' it.
func newDeployment(foo *example.Foo) *apps.Deployment {
	return &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      foo.Spec.DeploymentName,
			Namespace: foo.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(foo, schema.GroupVersionKind{
					Group:   example.SchemeGroupVersion.Group,
					Version: example.SchemeGroupVersion.Version,
					Kind:    "Foo",
				}),
			},
		},
		Spec: apps.DeploymentSpec{
			Replicas: foo.Spec.Replicas,
			Template: core.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":        "nginx",
						"controller": foo.Name,
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
