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
	"sort"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/printers"
)

var (
	apiresources_example = templates.Examples(`
		# Print the supported API Resources
		kubectl api-resources`)
)

// ApiResourcesOptions is the start of the data required to perform the operation.  As new fields are added, add them here instead of
// referencing the cmd.Flags()
type ApiResourcesOptions struct {
	out io.Writer

	Output     string
	APIGroup   string
	Namespaced bool
	NoHeaders  bool
}

// groupResource contains the APIGroup and APIResource
type groupResource struct {
	APIGroup    string
	APIResource metav1.APIResource
}

func NewCmdApiResources(f cmdutil.Factory, out io.Writer) *cobra.Command {
	options := &ApiResourcesOptions{
		out: out,
	}

	cmd := &cobra.Command{
		Use:     "api-resources",
		Short:   "Print the supported API resources on the server",
		Long:    "Print the supported API resources on the server",
		Example: apiresources_example,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(options.Validate(args))
			cmdutil.CheckErr(options.RunApiResources(cmd, f))
		},
	}
	cmd.Flags().StringVarP(&options.Output, "output", "o", "", "Output mode. Use \"-o wide\" for wide output.")
	cmd.Flags().StringVar(&options.APIGroup, "api-group", "", "The API group to use when talking to the server.")
	cmd.Flags().BoolVar(&options.Namespaced, "namespaced", true, "Namespaced indicates if a resource is namespaced or not.")
	cmd.Flags().BoolVar(&options.NoHeaders, "no-headers", false, "When using the default or custom-column output format, don't print headers (default print headers).")
	return cmd
}

func (o *ApiResourcesOptions) Validate(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("not require arguments.")
	}
	return nil
}

func (o *ApiResourcesOptions) RunApiResources(cmd *cobra.Command, f cmdutil.Factory) error {
	w := printers.GetNewTabWriter(o.out)
	defer w.Flush()

	discoveryclient, err := f.DiscoveryClient()
	if err != nil {
		return err
	}

	// Always request fresh data from the server
	discoveryclient.Invalidate()

	lists, err := discoveryclient.ServerPreferredResources()
	if err != nil {
		glog.V(1).Infof("Get available api resources from server failed: %v", err)
	}

	resources := []groupResource{}

	groupChanged := cmd.Flags().Changed("api-group")
	nsChanged := cmd.Flags().Changed("namespaced")

	for _, list := range lists {
		if len(list.APIResources) == 0 {
			continue
		}
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			glog.V(1).Infof("Unable to parse groupversion %s:%s", list.GroupVersion, err.Error())
			continue
		}
		for _, resource := range list.APIResources {
			if len(resource.Verbs) == 0 {
				continue
			}
			// filter apiGroup
			if groupChanged && o.APIGroup != gv.Group {
				continue
			}
			// filter namespaced
			if nsChanged && o.Namespaced != resource.Namespaced {
				continue
			}
			resources = append(resources, groupResource{
				APIGroup:    gv.Group,
				APIResource: resource,
			})
		}
	}

	if o.NoHeaders == false {
		if o.Output == "wide" {
			fmt.Fprintln(w, "NAME\tSHORTNAMES\tAPIGROUP\tNAMESPACED\tKIND\tVERBS")
		} else {
			fmt.Fprintln(w, "NAME\tSHORTNAMES\tAPIGROUP\tNAMESPACED\tKIND")
		}
	}

	sort.Stable(sortableGroupResource(resources))
	for _, r := range resources {
		if o.Output == "wide" {
			if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%v\t%s\t%v\n",
				r.APIResource.Name,
				strings.Join(r.APIResource.ShortNames, ","),
				r.APIGroup,
				r.APIResource.Namespaced,
				r.APIResource.Kind,
				r.APIResource.Verbs); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%v\t%s\n",
				r.APIResource.Name,
				strings.Join(r.APIResource.ShortNames, ","),
				r.APIGroup,
				r.APIResource.Namespaced,
				r.APIResource.Kind); err != nil {
				return err
			}
		}
	}
	return nil
}

type sortableGroupResource []groupResource

func (s sortableGroupResource) Len() int      { return len(s) }
func (s sortableGroupResource) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s sortableGroupResource) Less(i, j int) bool {
	ret := strings.Compare(s[i].APIGroup, s[j].APIGroup)
	if ret > 0 {
		return false
	} else if ret == 0 {
		return strings.Compare(s[i].APIResource.Name, s[j].APIResource.Name) < 0
	}
	return true
}
