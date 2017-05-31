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
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/test/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func preparePod(name, propagation, hostDir string) *v1.Pod {
	const containerName = "cntr"
	propagationJson := fmt.Sprintf(`{"%s": {"host": "%s"}}`, containerName, propagation)
	bTrue := true
	var oneSecond int64 = 1
	// The pod prepares /mnt/test/<podname> and sleeps
	cmd := fmt.Sprintf("mkdir /mnt/test/%[1]s; sleep 3600", name)
	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				"volume.alpha.kubernetes.io/propagation": propagationJson,
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:    containerName,
					Image:   "gcr.io/google_containers/busybox:1.24",
					Command: []string{"sh", "-c", cmd},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "host",
							MountPath: "/mnt/test",
						},
					},
					SecurityContext: &v1.SecurityContext{
						Privileged: &bTrue,
					},
				},
			},
			Volumes: []v1.Volume{
				{
					Name: "host",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: hostDir,
						},
					},
				},
			},
			// speed up termination of the pod
			TerminationGracePeriodSeconds: &oneSecond,
		},
	}
	return pod
}

var _ = framework.KubeDescribe("MountPropagation", func() {
	f := framework.NewDefaultFramework("mount-propagation-test")

	// This test runs three pods: master, slave and private with respective
	// mount propagation on common /var/lib/kubelet/XXXX directory. All of them
	// mount a tmpfs to a subdirectory there. We check that these mounts are
	// propagated to the right places.
	It("should propagate mounts to the host", func() {
		// Skip this test on Debian Wheezy. This it temporary workaround until
		// #45724 is merged.
		release, err := ioutil.ReadFile("/etc/os-release")
		if err == nil && strings.Contains(string(release), "wheezy") {
			framework.Skipf("Debian Wheezy does not support mount propagation")
		}
		// Ignore ReadFile error - it's unknown distro and we expect / as shared
		// there.

		// hostDir is the directory that's shared via HostPath among all pods.
		// Make sure it's random enough so we don't clash with another test
		// running in parallel.
		hostDir := "/var/lib/kubelet/" + f.Namespace.Name
		defer os.RemoveAll(hostDir)

		podClient := f.PodClient()
		master := podClient.CreateSync(preparePod("master", "rshared", hostDir))
		defer podClient.Delete(master.Name, nil)

		slave := podClient.CreateSync(preparePod("slave", "rslave", hostDir))
		defer podClient.Delete(slave.Name, nil)

		private := podClient.CreateSync(preparePod("private", "private", hostDir))
		defer podClient.Delete(private.Name, nil)

		// Check that the pods sees directories of each other. This just checks
		// that they have the same HostPath, not the mount propagation.
		podNames := []string{master.Name, slave.Name, private.Name}
		for _, podName := range podNames {
			for _, dirName := range podNames {
				cmd := fmt.Sprintf("test -d /mnt/test/%s", dirName)
				_ = f.ExecShellInPod(podName, cmd)
			}
		}

		// Each pod mounts one tmpfs to /mnt/test/<podname> and puts a file there
		for _, podName := range podNames {
			cmd := fmt.Sprintf("mount -t tmpfs e2e-mount-propagation-%[1]s /mnt/test/%[1]s; echo %[1]s > /mnt/test/%[1]s/file", podName)
			_ = f.ExecShellInPod(podName, cmd)
			cmd = fmt.Sprintf("umount /mnt/test/%s", podName)
			defer f.ExecShellInPod(podName, cmd)
		}

		// Now check that mounts are propagated to the right places.
		// expectedMounts is map of pod name -> expected mounts visible in the
		// pod.
		expectedMounts := map[string]map[string]bool{
			// Master sees only its own mount, neither slave nor private pods
			// can propagate theirs mounts to master.
			"master": {
				"master":  true,
				"slave":   false,
				"private": false,
			},
			// Slave sees master's mount + itself, it can't see private pod.
			"slave": {
				"master":  true,
				"slave":   true,
				"private": false,
			},
			// Private does not get any mounts from anywhere but its own
			"private": {
				"master":  false,
				"slave":   false,
				"private": true,
			},
		}
		for podName, mounts := range expectedMounts {
			for mountName, visible := range mounts {
				cmd := fmt.Sprintf("cat /mnt/test/%s/file", mountName)
				stdout, stderr, err := f.ExecShellInPodWithFullOutput(podName, cmd)
				framework.Logf("pod %s mount %s: stdout: %q, stderr: %q error: %v", podName, mountName, stdout, stderr, err)
				msg := fmt.Sprintf("When checking pod %s and directory %s", podName, mountName)
				if visible {
					Expect(err).NotTo(HaveOccurred(), "failed to run %q", cmd, msg)
					Expect(stdout, Equal(mountName), msg)
				} else {
					Expect(err).To(HaveOccurred(), msg)
				}
			}
		}
	})
})
