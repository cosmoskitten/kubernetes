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
	"os"
	"os/exec"
	"strconv"
	"strings"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/test/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func preparePod(name string, propagation v1.MountPropagationMode, hostDir string) *v1.Pod {
	const containerName = "cntr"
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
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:    containerName,
					Image:   busyboxImage,
					Command: []string{"sh", "-c", cmd},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:             "host",
							MountPath:        "/mnt/test",
							MountPropagation: &propagation,
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

	It("should propagate mounts to the host", func() {
		// This test runs three pods: master, slave and private with respective
		// mount propagation on common /var/lib/kubelet/XXXX directory. All of them
		// mount a tmpfs to a subdirectory there. We check that these mounts are
		// propagated to the right places.

		if err := checkMountPropagation(); err != nil {
			framework.Skipf("cannot test mount propagation: %v", err)
		}

		// hostDir is the directory that's shared via HostPath among all pods.
		// Make sure it's random enough so we don't clash with another test
		// running in parallel.
		hostDir := "/var/lib/kubelet/" + f.Namespace.Name
		defer os.RemoveAll(hostDir)

		podClient := f.PodClient()
		master := podClient.CreateSync(preparePod("master", v1.MountPropagationBidirectional, hostDir))
		defer podClient.Delete(master.Name, nil)

		slave := podClient.CreateSync(preparePod("slave", v1.MountPropagationHostToContainer, hostDir))
		defer podClient.Delete(slave.Name, nil)

		// Check that the pods sees directories of each other. This just checks
		// that they have the same HostPath, not the mount propagation.
		podNames := []string{master.Name, slave.Name}
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
		expectedMounts := map[string]sets.String{
			// Master sees only its own mount and not the slave's one.
			"master": sets.NewString("master"),
			// Slave sees master's mount + itself
			"slave": sets.NewString("master", "slave"),
		}
		for podName, mounts := range expectedMounts {
			for _, mountName := range podNames {
				cmd := fmt.Sprintf("cat /mnt/test/%s/file", mountName)
				stdout, stderr, err := f.ExecShellInPodWithFullOutput(podName, cmd)
				framework.Logf("pod %s mount %s: stdout: %q, stderr: %q error: %v", podName, mountName, stdout, stderr, err)
				msg := fmt.Sprintf("When checking pod %s and directory %s", podName, mountName)
				shouldBeVisible := mounts.Has(mountName)
				if shouldBeVisible {
					Expect(err).NotTo(HaveOccurred(), "failed to run %q", cmd, msg)
					Expect(stdout, Equal(mountName), msg)
				} else {
					Expect(err).To(HaveOccurred(), msg)
				}
			}
		}
	})
})

// checkMountPropagation returns error if kubernetes and the host operating
// system looks like it desn't support mount propagation.
func checkMountPropagation() error {
	// Check kubelet features
	cfg, err := getCurrentKubeletConfig()
	if err != nil {
		return err
	}
	if !strings.Contains(cfg.FeatureGates, "MountPropagation=true") {
		return fmt.Errorf("mount propagation is disabled by feature gate")
	}

	// Docker <= 1.12 runs docker daemon in a slave mount namespace ->
	// no mount propagation.
	const minimalMajor = 1
	const minimalMinor = 13

	cmd := exec.Command("docker", "version", "-f", "{{.Server.Version}}")
	out, err := cmd.Output()
	if err != nil {
		// Something failed, probably no docker on the host. Assume it does not
		// support mount propagation.
		return fmt.Errorf("error running 'docker version': %v", err)
	}
	version := string(out)
	components := strings.Split(version, ".")
	if len(components) < 2 {
		return fmt.Errorf("error parsing docker version %q: expected at least minor.major numbers", version)
	}
	major, err := strconv.Atoi(components[0])
	if err != nil {
		return fmt.Errorf("error parsing the major part of docker version %q: %v", version, err)
	}
	if major > minimalMajor {
		// version 2 and later do support mount propagation
		return nil
	}
	minor, err := strconv.Atoi(components[1])
	if err != nil {
		return fmt.Errorf("error parsing the minor part of docker version %q: %v", version, err)
	}
	if minor < minimalMinor {
		// version 1.12 and lower runs docker daemon in a slave namespace
		return fmt.Errorf("docker version %q is too low, at least %d.%d is required", version, minimalMajor, minimalMinor)
	}

	return nil
}
