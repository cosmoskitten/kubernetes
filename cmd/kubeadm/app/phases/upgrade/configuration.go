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

package upgrade

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	clientset "k8s.io/client-go/kubernetes"
	kubeadmapiext "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1alpha1"
	"k8s.io/kubernetes/pkg/api"
)

// FetchConfiguration fetches configuration required for upgrading your cluster from a file (which has precedence) or a ConfigMap in the cluster
func FetchConfiguration(_ clientset.Interface, _ io.Writer, _ string, _ bool) (*kubeadmapiext.MasterConfiguration, error) {
	fmt.Println("[upgrade/config] Making sure the configuration is correct:")

	cfg := &kubeadmapiext.MasterConfiguration{}
	api.Scheme.Default(cfg)

	return cfg, nil
}

// InteractivelyConfirmUpgrade asks the user whether they _really_ want to upgrade.
func InteractivelyConfirmUpgrade(question string) error {

	fmt.Printf("[upgrade/confirm] %s? [y/N]: ", question)

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("couldn't read from standard input: %v", err)
	}
	answer := scanner.Text()
	if strings.ToLower(answer) != "y" {
		return fmt.Errorf("won't proceed; the user didn't answer (Y|y) in order to continue")
	}

	return nil
}
