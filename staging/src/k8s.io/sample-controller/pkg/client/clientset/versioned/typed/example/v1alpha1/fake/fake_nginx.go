/*
Copyright 2017 The Kubernetes sample-controller Authors.

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

package fake

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
	v1alpha1 "k8s.io/sample-controller/pkg/apis/example/v1alpha1"
)

// FakeNGINXs implements NGINXInterface
type FakeNGINXs struct {
	Fake *FakeExampleV1alpha1
	ns   string
}

var nginxsResource = schema.GroupVersionResource{Group: "example.controller.code-generator.k8s.io", Version: "v1alpha1", Resource: "nginxs"}

var nginxsKind = schema.GroupVersionKind{Group: "example.controller.code-generator.k8s.io", Version: "v1alpha1", Kind: "NGINX"}

// Get takes name of the nGINX, and returns the corresponding nGINX object, and an error if there is any.
func (c *FakeNGINXs) Get(name string, options v1.GetOptions) (result *v1alpha1.NGINX, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(nginxsResource, c.ns, name), &v1alpha1.NGINX{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.NGINX), err
}

// List takes label and field selectors, and returns the list of NGINXs that match those selectors.
func (c *FakeNGINXs) List(opts v1.ListOptions) (result *v1alpha1.NGINXList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(nginxsResource, nginxsKind, c.ns, opts), &v1alpha1.NGINXList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.NGINXList{}
	for _, item := range obj.(*v1alpha1.NGINXList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested nGINXs.
func (c *FakeNGINXs) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(nginxsResource, c.ns, opts))

}

// Create takes the representation of a nGINX and creates it.  Returns the server's representation of the nGINX, and an error, if there is any.
func (c *FakeNGINXs) Create(nGINX *v1alpha1.NGINX) (result *v1alpha1.NGINX, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(nginxsResource, c.ns, nGINX), &v1alpha1.NGINX{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.NGINX), err
}

// Update takes the representation of a nGINX and updates it. Returns the server's representation of the nGINX, and an error, if there is any.
func (c *FakeNGINXs) Update(nGINX *v1alpha1.NGINX) (result *v1alpha1.NGINX, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(nginxsResource, c.ns, nGINX), &v1alpha1.NGINX{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.NGINX), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeNGINXs) UpdateStatus(nGINX *v1alpha1.NGINX) (*v1alpha1.NGINX, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(nginxsResource, "status", c.ns, nGINX), &v1alpha1.NGINX{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.NGINX), err
}

// Delete takes name of the nGINX and deletes it. Returns an error if one occurs.
func (c *FakeNGINXs) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(nginxsResource, c.ns, name), &v1alpha1.NGINX{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeNGINXs) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(nginxsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.NGINXList{})
	return err
}

// Patch applies the patch and returns the patched nGINX.
func (c *FakeNGINXs) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.NGINX, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(nginxsResource, c.ns, name, data, subresources...), &v1alpha1.NGINX{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.NGINX), err
}
