<!-- BEGIN MUNGE: UNVERSIONED_WARNING -->

<!-- BEGIN STRIP_FOR_RELEASE -->

<img src="http://kubernetes.io/img/warning.png" alt="WARNING"
     width="25" height="25">
<img src="http://kubernetes.io/img/warning.png" alt="WARNING"
     width="25" height="25">
<img src="http://kubernetes.io/img/warning.png" alt="WARNING"
     width="25" height="25">
<img src="http://kubernetes.io/img/warning.png" alt="WARNING"
     width="25" height="25">
<img src="http://kubernetes.io/img/warning.png" alt="WARNING"
     width="25" height="25">

<h2>PLEASE NOTE: This document applies to the HEAD of the source tree</h2>

If you are using a released version of Kubernetes, you should
refer to the docs that go with that version.

<!-- TAG RELEASE_LINK, added by the munger automatically -->
<strong>
The latest release of this document can be found
[here](http://releases.k8s.io/release-1.3/docs/proposals/custom-metrics.md).

Documentation for other releases can be found at
[releases.k8s.io](http://releases.k8s.io).
</strong>
--

<!-- END STRIP_FOR_RELEASE -->

<!-- END MUNGE: UNVERSIONED_WARNING -->

Specifying Metrics in the HPA
=============================

Introduction
------------

Currently, custom metrics specified via a list of metric names
and target values.  Each metric name is prefixed with "custom/",
and then is requested at the pod level for each pod of the target
scalable.

Several use cases are not supported by this design:

### Refering to Non-Custom Metrics ###

Several users have asked for the ability to scale on existing built-in metrics
(e.g. memory usage).  While the current design does allow the user to specify
a metric name, all metrics are prefixed with "custom/", meaning built-in
metrics cannot be passed this way.  A new design should not have completely
separate mechanisms for custom and non-custom metrics.  This also allows
refering to raw CPU usage rate, instead of refering to CPU usage as a
percentage of the CPU request (this has been a commonly asked-for feature
by users and operators).

### Refering to Metrics Not Associated With Pods ###

Heapster current can track metrics associated with pods and namespaces.  With
Heapster push metrics, it becomes possible to define custom metrics on the
namespace level.  However, the custom metrics annotation in the HPA currently
only allows referring to custom metrics defined on pods.

Refering to custom metrics at the namespace level allows us to support certain
use cases in which an application wishes to expose a certain special metric
to an entire namespace (e.g. queue length) and scale based on that.
Additionally, until Heapster has the capability to track metrics from service
objects, RCs, etc, namespace-level metrics can be used with push metrics to
"fake" this type of metrics.  Metrics associated with services could be
extracted from a load balancer or reverse proxy, for instance.

Heapster will, most likely, support metrics associated with services,
replicationcontrollers, etc, in the future, so it becomes adventageous to be
able to refer to metrics associated with those objects as well.

Proposed New Design
-------------------

```go
type SourceType string
var (
    SourceTypeNamespace SourceType = "namespace"  // the current namespace
    SourceTypePod SourceType = "pod"  // each pod that would be considered
    SourceTypeController SourceType = "controller" // the target of the HPA
)

type MetricSource struct {
    // Draw from the current namespace, pod, or controller
    CurrentSource SourceType `json:"current,omitempty`

    // Draw from an object in the current namespace (not currently supported
    // by Heapster)
    SourceRef *CrossVersionObjectReference `json:"object,omitempty"`
}

type MetricTarget struct {
    // users could either specify
    // `custom: my-metric` or `name: custom/my-metric`
    // builtin metrics would be specified with `name: cpu/usage`
    CustomName string `json:"custom,omitempty"`
    BuiltinName string `json:"name,omitempty"`

    TargetValue resource.Quantity `json:"targetValue"`

    // Specifies the target object
    // (missing is equivalent to {"current": "pod"})
    SourceObject *MetricSource `json:"from,omitempty"`
}

type HorizontalPodAutoscalerSpec struct {
    ScaleTargetRef CrossVersionObjectReference `json:"scaleTargetRef"`
    MinReplicas *int32 `json:"minReplicas,omitempty"`
    MaxReplicas int32 `json:"maxReplicas"`
    Metrics []MetricTarget `json:"metrics"`
}
```

This design allows us to refer to both custom metrics as well as built-in
metrics, and allows referring to metrics from the current namespace, metrics
from each pod individually (the default, and current setup), as well as metrics
associated with other objects in the namespace.

An HPA under this design would look like:

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
    ...
spec:
    scaleTargetRef:
        ...
    maxReplicas: 10
    metrics:
    - name: memory/usage
      targetValue: 256Mi
    - custom: my-pod-metric
      targetValue: 20
    - custom: special-namespace-app-metric
      from:
        current: "namespace"
    - custom: special-service-metric
      from:
        object:
            kind: Service
            apiVersion: v1
            name: "my-service"
```

This would result in the following queries to Heapster (or a similar API):

- `/api/v1/model/namespace/$NS/pod-list/$POD1,$POD2/metrics/memory/usage`
- `/api/v1/model/namespace/$NS/pod-list/$POD1,$POD2/metrics/custom/my-pod-metric`
- `/api/v1/model/namespace/$NS/metrics/custom/special-namespace-app-metric`
- `/api/v1/model/namespace/$NS/service/my-service/metrics/custom/special-service-metric`
  (not currently supported by Heapster, probably will be in the future)

<!-- BEGIN MUNGE: GENERATED_ANALYTICS -->
[![Analytics](https://kubernetes-site.appspot.com/UA-36037335-10/GitHub/docs/proposals/custom-metrics.md?pixel)]()
<!-- END MUNGE: GENERATED_ANALYTICS -->
