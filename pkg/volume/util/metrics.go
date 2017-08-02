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

package util

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var StorageOperationMetric = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name: "storage_operation_duration_seconds",
		Help: "Storage operation duration",
	},
	[]string{"volume_plugin", "operation_name"},
)

var StorageOperationErrorMetric = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "storage_operation_errors_total",
		Help: "Storage operation errors",
	},
	[]string{"volume_plugin", "operation_name"},
)

var registerMetrics sync.Once

func RegisterMetrics() {
	registerMetrics.Do(func() {
		prometheus.MustRegister(StorageOperationMetric)
		prometheus.MustRegister(StorageOperationErrorMetric)
	})
}

// OperationCompleteHook returns a hook to call when an operation is completed
func OperationCompleteHook(plugin, operationName string) func(error) {
	requestTime := time.Now()
	opComplete := func(err error) {
		timeTaken := time.Since(requestTime).Seconds()
		// Create metric with operation name and plugin name
		if err != nil {
			StorageOperationErrorMetric.WithLabelValues(plugin, operationName).Inc()
		} else {
			StorageOperationMetric.WithLabelValues(plugin, operationName).Observe(timeTaken)
		}
	}
	return opComplete
}
