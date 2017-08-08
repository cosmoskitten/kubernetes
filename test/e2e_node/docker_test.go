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

package e2e_node

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = framework.KubeDescribe("Docker features [Feature:Docker]", func() {
	f := framework.NewDefaultFramework("docker-feature-test")

	BeforeEach(func() {
		framework.RunIfContainerRuntimeIs("docker")
	})

	Context("when shared PID namespace is enabled", func() {
		It("processes in different containers of the same pod should be able to see each other", func() {
			// TODO(yguo0905): Change this test to run unless the runtime is
			// Docker and its version is <1.13.
			By("Check whether shared PID namespace is enabled.")
			isEnabled, err := isSharedPIDNamespaceEnabled()
			framework.ExpectNoError(err)
			if !isEnabled {
				framework.Skipf("Skipped because shared PID namespace is not enabled.")
			}

			By("Create a pod with two containers.")
			f.PodClient().CreateSync(&v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "shared-pid-ns-test-pod"},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:    "test-container-1",
							Image:   "gcr.io/google_containers/busybox:1.24",
							Command: []string{"/bin/top"},
						},
						{
							Name:    "test-container-2",
							Image:   "gcr.io/google_containers/busybox:1.24",
							Command: []string{"/bin/sleep"},
							Args:    []string{"10000"},
						},
					},
				},
			})

			By("Check if the process in one container is visible to the process in the other.")
			pid1 := f.ExecCommandInContainer("shared-pid-ns-test-pod", "test-container-1", "/bin/pidof", "top")
			pid2 := f.ExecCommandInContainer("shared-pid-ns-test-pod", "test-container-2", "/bin/pidof", "top")
			if pid1 != pid2 {
				framework.Failf("PIDs are not the same in different containers: test-container-1=%v, test-container-2=%v", pid1, pid2)
			}
		})
	})

	Context("when live-restore is enabled [Serial] [Slow] [Disruptive]", func() {
		It("containers should not be disrupted when the daemon is down", func() {
			const (
				podName           = "live-restore-test-pod"
				containerName     = "live-restore-test-container"
				volumeName        = "live-restore-test-volume"
				timestampFilename = "timestamp"
			)

			isSupported, err := isDockerLiveRestoreSupported()
			framework.ExpectNoError(err)
			if !isSupported {
				framework.Skipf("Docker live-restore is not supported.")
			}

			By("Check whether live-restore is enabled.")
			isEnabled, err := isDockerLiveRestoreEnabled()
			framework.ExpectNoError(err)
			framework.Logf("Docker live-restore enabled: %t", isEnabled)
			if !isEnabled {
				// Enables Docker live-restore if it's supported but disabled.
				By("Enable Docker live-restore.")
				framework.ExpectNoError(setDockerLiveRestore(true))

				b, err := isDockerLiveRestoreEnabled()
				framework.ExpectNoError(err)
				Expect(b).To(BeTrue(), "should be able to enable Docker live-restore")

				defer func() {
					By("Disable Docker live-restore.")
					time.Sleep(2 * time.Second)
					framework.ExpectNoError(setDockerLiveRestore(false))
				}()
			}

			// Creates a temporary directory that will be mounted into the
			// container, serving as the communication channel between the host
			// and the container.
			By("Create temporary directory for mount.")
			tempDir, err := ioutil.TempDir("", "")
			framework.ExpectNoError(err)
			defer func() {
				By("Remove temporary directory.")
				os.RemoveAll(tempDir)
			}()

			// Creates a container that writes the current timestamp every
			// second to the timestamp file. We will be able to tell whether
			// the container is running by checking if the timestamp increases.
			cmd := "" +
				"while true; do " +
				"    date +%s > /test-dir/" + timestampFilename + "; " +
				"    sleep 1; " +
				"done"
			By("Create the test pod.")
			f.PodClient().CreateSync(&v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: podName},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Name:    containerName,
						Image:   "gcr.io/google_containers/busybox:1.24",
						Command: []string{"/bin/sh"},
						Args:    []string{"-c", cmd},
						VolumeMounts: []v1.VolumeMount{
							{
								Name:      volumeName,
								MountPath: "/test-dir",
							},
						},
					}},
					Volumes: []v1.Volume{{
						Name: volumeName,
						VolumeSource: v1.VolumeSource{
							HostPath: &v1.HostPathVolumeSource{Path: tempDir},
						},
					}},
				},
			})

			startTime1, err := getContainerStartTime(f, podName, containerName)
			framework.ExpectNoError(err)

			By("Stop Docker daemon.")
			framework.ExpectNoError(stopDockerDaemon())
			defer func() {
				By("Restart Docker daemon.")
				framework.ExpectNoError(startDockerDaemon())
			}()

			By("Ensure that the test container is running when Docker daemon is down.")
			isRunning, err := isContainerRunning(filepath.Join(tempDir, timestampFilename))
			framework.ExpectNoError(err)
			if !isRunning {
				framework.Failf("The container should be running but it's not.")
			}

			By("Ensure that the test pod is still running when Docker daemon is down.")
			framework.ExpectNoError(f.WaitForPodRunning(podName))

			By("Start Docker daemon.")
			framework.ExpectNoError(startDockerDaemon())

			By("Ensure that the test container is running after Docker daemon is restarted.")
			isRunning, err = isContainerRunning(filepath.Join(tempDir, timestampFilename))
			framework.ExpectNoError(err)
			if !isRunning {
				framework.Failf("The container should be running but it's not.")
			}

			By("Ensure that the test container has not been restarted after Docker daemon is restarted.")
			startTime2, err := getContainerStartTime(f, podName, containerName)
			framework.ExpectNoError(err)
			if startTime1 != startTime2 {
				framework.Failf("The container should have not been restarted.")
			}
		})
	})
})

// isContainerRunning returns true if the container is running (by checking
// whether the timestamp is being updated), and false otherwise. Returns an
// error if the timestamp cannot be read.
func isContainerRunning(filename string) (bool, error) {
	const (
		// The sample interval (3s), which must be greater than the interval at
		// which the container writes the timestamp (every second).
		sampleInterval = 3 * time.Second
		retryInterval  = 3 * time.Second
		retryTimeout   = 30 * time.Second
	)
	for start := time.Now(); time.Since(start) < retryTimeout; time.Sleep(retryInterval) {
		c1, err := getTimestamp(filename)
		if err != nil {
			return false, err
		}
		time.Sleep(sampleInterval)
		c2, err := getTimestamp(filename)
		if err != nil {
			return false, err
		}
		if c1 != c2 {
			return true, nil
		}
	}
	return false, nil
}

// getTimestamp returns the timestamp in the file with the specified filename,
// and false if the timestamp cannot be read.
func getTimestamp(filename string) (int, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return 0, err
	}
	c, err := strconv.Atoi(string(bytes.Trim(data, "\n")))
	if err != nil {
		return 0, err
	}
	return c, nil
}

// getContainerStartTime returns the start time of the container with the
// containerName of the pod having the podName.
func getContainerStartTime(f *framework.Framework, podName, containerName string) (time.Time, error) {
	pod, err := f.PodClient().Get(podName, metav1.GetOptions{})
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get pod %q", podName)
	}
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name != containerName {
			continue
		}
		if status.State.Running == nil {
			return time.Time{}, fmt.Errorf("%v/%v is not running", podName, containerName)
		}
		return status.State.Running.StartedAt.Time, nil
	}
	return time.Time{}, fmt.Errorf("failed to find %v/%v", podName, containerName)
}
