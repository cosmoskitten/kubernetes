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

package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/util/i18n"
)

var (
	daemonsetLong = templates.LongDesc(i18n.T(`
	Create a daemonset with the specified name.

	A DaemonSet ensures that all (or some) nodes run a copy of a pod.  As nodes are added to the
	cluster, pods are added to them.  As nodes are removed from the cluster, those pods are garbage
	collected.  Deleting a DaemonSet will clean up the pods it created.

	Some typical uses of a DaemonSet are:

	- running a cluster storage daemon, such as 'glusterd', 'ceph', on each node.
	- running a logs collection daemon on every node, such as 'fluentd' or 'logstash'.
	- running a node monitoring daemon on every node, such as [Prometheus Node Exporter](
	  https://github.com/prometheus/node_exporter), 'collectd', Datadog agent, New Relic agent, or Ganglia 'gmond'.

	In a simple case, one DaemonSet, covering all nodes, would be used for each type of daemon.
	A more complex setup might use multiple DaemonSets for a single type of daemon, but with
	different flags and/or different memory and cpu requests for different hardware types.

	See more detail information [https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/]
	`))

	daemonsetExample = templates.Examples(i18n.T(`
	# Create a new daemonset named my-dea that runs the busybox image.
	kubectl create daemonset my-dea --image=busybox`))
)

// NewCmdCreateDaemonset is a command to create a new daemonset.
func NewCmdCreateDaemonset(f cmdutil.Factory, cmdOut, cmdErr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "daemonset NAME --image=image [--dry-run]",
		Aliases: []string{"ds"},
		Short:   i18n.T("Create a daemonset with the specified name."),
		Long:    daemonsetLong,
		Example: daemonsetExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(CreateDaemonset(f, cmdOut, cmdErr, cmd, args))
		},
	}
	cmdutil.AddApplyAnnotationFlags(cmd)
	cmdutil.AddValidateFlags(cmd)
	cmdutil.AddPrinterFlags(cmd)
	cmdutil.AddGeneratorFlags(cmd, cmdutil.DaemonsetAppsV1Beta2GeneratorName)
	cmd.Flags().StringSlice("image", []string{}, "Image name to run.")
	cmd.MarkFlagRequired("image")
	return cmd
}

func CreateDaemonset(f cmdutil.Factory, cmdOut, cmdErr io.Writer, cmd *cobra.Command, args []string) error {
	name, err := NameFromCommandArgs(cmd, args)
	if err != nil {
		return err
	}
	generatorName := cmdutil.GetFlagString(cmd, "generator")
	image := cmdutil.GetFlagStringSlice(cmd, "image")

	clientset, err := f.ClientSet()
	if err != nil {
		return err
	}
	resourcesList, err := clientset.Discovery().ServerResources()
	// ServerResources ignores errors for old servers do not expose discovery
	if err != nil {
		return fmt.Errorf("failed to discover supported resources: %v", err)
	}

	// It is possible we have to modify the user-provided generator name if
	// the server does not have support for the requested generator.
	generatorName = cmdutil.FallbackGeneratorNameIfNecessary(generatorName, resourcesList, cmdErr)

	var generator kubectl.StructuredGenerator
	switch generatorName {
	case cmdutil.DaemonsetExtensionsV1Beta1GeneratorName:
		generator = &kubectl.DaemonSetGeneratorExtensionsV1Beta1{
			BaseGenerator: kubectl.BaseGenerator{
				Name:   name,
				Images: image,
			},
		}
	case cmdutil.DaemonsetAppsV1Beta2GeneratorName:
		generator = &kubectl.DaemonSetGeneratorAppsV1Beta2{
			BaseGenerator: kubectl.BaseGenerator{
				Name:   name,
				Images: image,
			},
		}
	default:
		return errUnsupportedGenerator(cmd, generatorName)
	}
	return RunCreateSubcommand(f, cmd, cmdOut, &CreateSubcommandOptions{
		Name:                name,
		StructuredGenerator: generator,
		DryRun:              cmdutil.GetDryRunFlag(cmd),
		OutputFormat:        cmdutil.GetFlagString(cmd, "output"),
	})
}
