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

package imagemanifest

import (
	"runtime"
)

const (
	testRegistry                   = "gcr.io/kubernetes-e2e-test-images"
	clusterTesterImageName         = "clusterapi-tester"
	clusterTesterImageVersion      = "1.0"
	cudaVectorAddImageName         = "cuda-vector-add"
	cudaVectorAddImageVersion      = "1.0"
	dnsutilsImageName              = "dnsutils"
	dnsutilsImageVersion           = "1.0"
	entrypointTesterImageName      = "entrypoint-tester"
	entrypointTesterImageVersion   = "1.0"
	fakegitserverImageName         = "fakegitserver"
	fakegitserverImageVersion      = "1.0"
	frontendImageName              = "frontend"
	frontendImageVersion           = "1.0"
	goproxyImageName               = "goproxy"
	goproxyImageVersion            = "1.0"
	hostexecImageName              = "hostexec"
	hostexecImageVersion           = "1.0"
	iperfImageName                 = "iperf"
	iperfImageVersion              = "1.0"
	jessieDnsutilsImageName        = "jessie-dnsutils"
	jessieDnsutilsImageVersion     = "1.0"
	kittenImageName                = "kitten"
	kittenImageVersion             = "1.0"
	livenessImageName              = "liveness"
	livenessImageVersion           = "1.0"
	logsGeneratorImageName         = "logs-generator"
	logsGeneratorImageVersion      = "1.0"
	mounttestImageName             = "mounttest"
	mounttestImageVersion          = "1.0"
	mounttestUserImageName         = "mounttest-user"
	mounttestUserImageVersion      = "1.0"
	nautilusImageName              = "nautilus"
	nautilusImageVersion           = "1.0"
	netImageName                   = "net"
	netImageVersion                = "1.0"
	netexecImageName               = "netexec"
	netexecImageVersion            = "1.0"
	nettestImageName               = "nettest"
	nettestImageVersion            = "1.0"
	nginxSlimImageName             = "nginx-slim"
	nginxSlimImageVersion          = "0.20"
	nginxSlimNewImageVersion       = "0.21"
	noSnatTestImageName            = "no-snat-test"
	noSnatTestImageVersion         = "1.0"
	noSnatTestProxyImageName       = "no-snat-test-proxy"
	noSnatTestProxyImageVersion    = "1.0"
	nWayHTTPImageName              = "n-way-http"
	nWayHTTPImageVersion           = "1.0"
	pauseImageName                 = "pause"
	pauseImageVersion              = "3.0"
	porterImageName                = "porter"
	porterImageVersion             = "1.0"
	portForwardTesterImageName     = "port-forward-tester"
	portForwardTesterImageVersion  = "1.0"
	redisImageName                 = "redis"
	redisImageVersion              = "1.0"
	redisslaveImageName            = "redis-slave"
	redisslaveImageVersion         = "1.0"
	resourceConsumerImageName      = "resource-consumer"
	resourceConsumerImageVersion   = "1.0"
	resourceControllerImageName    = "resource-consumer/controller"
	resourceControllerImageVersion = "1.0"
	serveHostnameImageName         = "serve-hostname"
	serveHostnameImageVersion      = "1.0"
	testWebserverImageName         = "test-webserver"
	testWebserverImageVersion      = "1.0"
)

func getMultiArchImageNameWithVersion(registry, imageName, imageVersion string) string {
	return registry + "/" + imageName + "-" + runtime.GOARCH + ":" + imageVersion
}

// GetBusyboxImage returns multi-arch busybox docker image
func GetBusyboxImage() string {
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

// GetclusterTesterImage returns multi-arch cluster-image docker image
func GetclusterTesterImage() string {
	return getMultiArchImageNameWithVersion(testRegistry, clusterTesterImageName, clusterTesterImageVersion)
}

// GetcudaVectorAddImage returns multi-arch cuda-vector-add docker image
func GetcudaVectorAddImage() string {
	return getMultiArchImageNameWithVersion(testRegistry, cudaVectorAddImageName, cudaVectorAddImageVersion)
}

// GetdnsutilsImage returns multi-arch dnsutils docker image
func GetdnsutilsImage() string {
	return getMultiArchImageNameWithVersion(testRegistry, dnsutilsImageName, dnsutilsImageVersion)
}

// GetentrypointTesterImage returns multi-arch entrypoint-tester docker image
func GetentrypointTesterImage() string {
	return getMultiArchImageNameWithVersion(testRegistry, entrypointTesterImageName, entrypointTesterImageVersion)
}

// GetfakegitserverImage returns multi-arch fakegitserver docker image
func GetfakegitserverImage() string {
	return getMultiArchImageNameWithVersion(testRegistry, fakegitserverImageName, fakegitserverImageVersion)
}

// GetfrontendImage returns multi-arch frontend docker image
func GetfrontendImage() string {
	return getMultiArchImageNameWithVersion(testRegistry, frontendImageName, frontendImageVersion)
}

// GetgoproxyImage returns multi-arch goproxy docker image
func GetgoproxyImage() string {
	return getMultiArchImageNameWithVersion(testRegistry, goproxyImageName, goproxyImageVersion)
}

// GethostexecImage returns multi-arch hostexec docker image
func GethostexecImage() string {
	return getMultiArchImageNameWithVersion(testRegistry, hostexecImageName, hostexecImageVersion)
}

// GetiperfImage returns multi-arch iperf docker image
func GetiperfImage() string {
	return getMultiArchImageNameWithVersion(testRegistry, iperfImageName, iperfImageVersion)
}

// GetjessieDnsutilsImage returns multi-arch jessie-dnsutils docker image
func GetjessieDnsutilsImage() string {
	return getMultiArchImageNameWithVersion(testRegistry, jessieDnsutilsImageName, jessieDnsutilsImageVersion)
}

// GetlivenessImage returns multi-arch docker liveness image
func GetlivenessImage() string {
	return getMultiArchImageNameWithVersion(testRegistry, livenessImageName, livenessImageVersion)
}

// GetkittenImage returns multi-arch kitten docker image
func GetkittenImage() string {
	return getMultiArchImageNameWithVersion(testRegistry, kittenImageName, kittenImageVersion)
}

// GetlogsGeneratorImage returns multi-arch logs-generator docker image
func GetlogsGeneratorImage() string {
	return getMultiArchImageNameWithVersion(testRegistry, logsGeneratorImageName, logsGeneratorImageVersion)
}

// GetmounttestImage returns multi-arch mounttest docker image
func GetmounttestImage() string {
	return getMultiArchImageNameWithVersion(testRegistry, mounttestImageName, mounttestImageVersion)
}

// GetmounttestUserImage returns multi-arch mounttest-user docker image
func GetmounttestUserImage() string {
	return getMultiArchImageNameWithVersion(testRegistry, mounttestUserImageName, mounttestUserImageVersion)
}

// GetnetImage returns multi-arch net docker image
func GetnetImage() string {
	return getMultiArchImageNameWithVersion(testRegistry, netImageName, netImageVersion)
}

// GetnautilusImage returns multi-arch nautilus docker image
func GetnautilusImage() string {
	return getMultiArchImageNameWithVersion(testRegistry, nautilusImageName, nautilusImageVersion)
}

// GetnetexecImage returns multi-arch netexec docker image
func GetnetexecImage() string {
	return getMultiArchImageNameWithVersion(testRegistry, netexecImageName, netexecImageVersion)
}

// GetnettestImage returns multi-arch nettest docker image
func GetnettestImage() string {
	return getMultiArchImageNameWithVersion(testRegistry, nettestImageName, nettestImageVersion)
}

// GetnginxSlimImage returns multi-arch nginx-slim docker image
func GetnginxSlimImage() string {
	return getMultiArchImageNameWithVersion("gcr.io/google-containers", nginxSlimImageName, nginxSlimImageVersion)
}

// GetNewnginxSlimImage returns newer multi-arch nginx-slim docker image
func GetNewnginxSlimImage() string {
	return getMultiArchImageNameWithVersion("gcr.io/google-containers", nginxSlimImageName, nginxSlimNewImageVersion)
}

// GetnoSnatTestImage returns multi-arch no-snat-test docker image
func GetnoSnatTestImage() string {
	return getMultiArchImageNameWithVersion(testRegistry, noSnatTestImageName, noSnatTestImageVersion)
}

// GetnoSnatTestProxyImage returns multi-arch no-snat-test-proxy docker image
func GetnoSnatTestProxyImage() string {
	return getMultiArchImageNameWithVersion(testRegistry, noSnatTestProxyImageName, noSnatTestProxyImageVersion)
}

// GetnWayHTTPImage returns multi-arch n-way-http docker image
func GetnWayHTTPImage() string {
	return getMultiArchImageNameWithVersion(testRegistry, nWayHTTPImageName, nWayHTTPImageVersion)
}

// GetpauseImage returns multi-arch pause docker image
func GetpauseImage() string {
	return getMultiArchImageNameWithVersion("gcr.io/google-containers", pauseImageName, pauseImageVersion)
}

// GetporterImage returns multi-arch porter docker image
func GetporterImage() string {
	return getMultiArchImageNameWithVersion(testRegistry, porterImageName, porterImageVersion)
}

// GetportForwardTesterImage returns multi-arch port-forward-tester docker image
func GetportForwardTesterImage() string {
	return getMultiArchImageNameWithVersion(testRegistry, portForwardTesterImageName, portForwardTesterImageVersion)
}

// GetredisImage returns multi-arch redis docker image
func GetredisImage() string {
	return getMultiArchImageNameWithVersion(testRegistry, redisImageName, redisImageVersion)
}

// GetredisslaveImage returns multi-arch redisslave docker image
func GetredisslaveImage() string {
	return getMultiArchImageNameWithVersion(testRegistry, redisslaveImageName, redisslaveImageVersion)
}

// GetresourceConsumerImage returns multi-arch resource-consumer docker image
func GetresourceConsumerImage() string {
	return getMultiArchImageNameWithVersion(testRegistry, resourceConsumerImageName, resourceConsumerImageVersion)
}

// GetresourceControllerImage returns multi-arch resource-consumer/controller docker image
func GetresourceControllerImage() string {
	return getMultiArchImageNameWithVersion(testRegistry, resourceControllerImageName, resourceControllerImageVersion)
}

// GetserveHostnameImage returns multi-arch serve-hostname docker image
func GetserveHostnameImage() string {
	return getMultiArchImageNameWithVersion(testRegistry, serveHostnameImageName, serveHostnameImageVersion)
}

// GettestWebserverImage returns multi-arch test-webserver docker image
func GettestWebserverImage() string {
	return getMultiArchImageNameWithVersion(testRegistry, testWebserverImageName, testWebserverImageVersion)
}
