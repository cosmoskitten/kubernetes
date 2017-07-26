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

package image

import (
	"fmt"
	"runtime"
)

const (
	e2eImagesRegistry  = "gcr.io/kubernetes-e2e-test-images"
	BusyBox            = "busybox"
	ClusterTester      = "clusterapi-tester"
	CudaVectorAdd      = "cuda-vector-add"
	Dnsutils           = "dnsutils"
	EntrypointTester   = "entrypoint-tester"
	Fakegitserver      = "fakegitserver"
	Frontend           = "frontend"
	Goproxy            = "goproxy"
	Hostexec           = "hostexec"
	Iperf              = "iperf"
	JessieDnsutils     = "jessie-dnsutils"
	Kitten             = "kitten"
	Liveness           = "liveness"
	LogsGenerator      = "logs-generator"
	Mounttest          = "mounttest"
	MounttestUser      = "mounttest-user"
	Nautilus           = "nautilus"
	Net                = "net"
	Netexec            = "netexec"
	Nettest            = "nettest"
	NginxSlim          = "nginx-slim"
	NginxSlimNew       = "nginx-slim-new"
	NoSnatTest         = "no-snat-test"
	NoSnatTestProxy    = "no-snat-test-proxy"
	NWayHTTP           = "n-way-http"
	Pause              = "pause"
	Porter             = "porter"
	PortForwardTester  = "port-forward-tester"
	Redis              = "redis"
	Redisslave         = "redis-slave"
	ResourceConsumer   = "resource-consumer"
	ResourceController = "resource-consumer/controller"
	ServeHostname      = "serve-hostname"
	TestWebserver      = "test-webserver"
)

var imageVersions = map[string]string{
	ClusterTester:      "1.0",
	CudaVectorAdd:      "1.0",
	Dnsutils:           "1.0",
	EntrypointTester:   "1.0",
	Fakegitserver:      "1.0",
	Frontend:           "1.0",
	Goproxy:            "1.0",
	Hostexec:           "1.0",
	Iperf:              "1.0",
	JessieDnsutils:     "1.0",
	Kitten:             "1.0",
	Liveness:           "1.0",
	LogsGenerator:      "1.0",
	Mounttest:          "1.0",
	MounttestUser:      "1.0",
	Nautilus:           "1.0",
	Net:                "1.0",
	Netexec:            "1.0",
	Nettest:            "1.0",
	NginxSlim:          "0.20",
	NginxSlimNew:       "0.21",
	NoSnatTest:         "1.0",
	NoSnatTestProxy:    "1.0",
	NWayHTTP:           "1.0",
	Pause:              "3.0",
	Porter:             "1.0",
	PortForwardTester:  "1.0",
	Redis:              "1.0",
	Redisslave:         "1.0",
	ResourceConsumer:   "1.0",
	ResourceController: "1.0",
	ServeHostname:      "1.0",
	TestWebserver:      "1.0",
}

var imageExceptions = map[string](func() string){
	BusyBox:      getBusyBoxImage,
	NginxSlimNew: getNginxSlimNewImage,
	NginxSlim:    getNginxSlimImage,
	Pause:        getPauseImage,
}

func GetE2EImage(image string) string {
	if nameFunc, ok := imageExceptions[image]; ok {
		return nameFunc()
	}

	return fmt.Sprintf("%s/%s-%s:%s", e2eImagesRegistry, image, runtime.GOARCH, imageVersions[image])
}

func getNginxSlimNewImage() string {
	return fmt.Sprintf("%s/%s-%s:%s", "gcr.io/google-containers", NginxSlim, runtime.GOARCH, imageVersions[NginxSlimNew])
}

func getNginxSlimImage() string {
	return fmt.Sprintf("%s/%s-%s:%s", "gcr.io/google-containers", NginxSlim, runtime.GOARCH, imageVersions[NginxSlim])
}

// GetBusyboxImage returns multi-arch busybox docker image
func getBusyBoxImage() string {
	switch arch := runtime.GOARCH; arch {
	case "amd64":
		return "busybox"
	case "arm":
		return "arm32v6/busybox"
	case "arm64":
		return "arm64v8/busybox"
	case "ppc64le":
		return "ppc64le/busybox"
	case "s390x":
		return "s390x/busybox"
	default:
		return ""
	}
}

func getPauseImage() string {
	return fmt.Sprintf("%s/%s-%s:%s", "gcr.io/google-containers", Pause, runtime.GOARCH, imageVersions[Pause])
}
