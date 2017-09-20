# sample-controller

This repository implements a simple controller for watching Foo resources as
defined with a CustomResourceDefinition (CRD).

It makes use of the generators in [k8s.io/code-generate](https://github.com/kubernetes/code-generator)
to generate a typed client, informers, listers and deep copy functions. You can
do this yourself using the `./hack/update-codegen.sh` script.

# Purpose

This is an example of how to build a kube-like controller with a single type.

# Compatibility

HEAD of this repository will match HEAD of k8s.io/apimachinery and
k8s.io/client-go.

# Where does it come from?

`sample-controller` is synced from
https://github.com/kubernetes/kubernetes/blob/master/staging/src/k8s.io/sample-controller.
Code changes are made in that location, merged into k8s.io/kubernetes and
later synced here.
