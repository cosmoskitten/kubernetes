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

	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	lists, err := discoveryclient.ServerPreferredResources()
	if err != nil {
		return fmt.Errorf("Couldn't get available api resources from server: %v", err)
	}

	resources := []groupResource{}
	changed := cmd.Flags().Changed("namespaced")

	for _, list := range lists {
		if len(list.APIResources) == 0 {
			continue
		}
		parts := strings.SplitN(list.GroupVersion, "/", 2)
		for _, resource := range list.APIResources {
			if len(resource.Verbs) == 0 {
				continue
			}
			// filter apiGroup
			if o.APIGroup != "" && o.APIGroup != parts[0] {
				continue
			}
			// filter namespaced
			if changed && o.Namespaced != resource.Namespaced {
				continue
			}
			resources = append(resources, groupResource{
				APIGroup:    parts[0],
				APIResource: resource,
			})
		}
	}

	if o.NoHeaders == false {
		fmt.Fprintln(w, "NAME\tNAMESPACED\tAPIGROUP\tKIND\tVERBS")
	}

	sort.Stable(sortableGroupResource(resources))
	for _, r := range resources {
		if _, err := fmt.Fprintf(w, "%s\t%v\t%s\t%s\t%v\n",
			r.APIResource.Name,
			r.APIResource.Namespaced,
			r.APIGroup,
			r.APIResource.Kind,
			r.APIResource.Verbs); err != nil {
			return err
		}
	}
	return nil
}

type sortableGroupResource []groupResource

func (s sortableGroupResource) Len() int      { return len(s) }
func (s sortableGroupResource) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s sortableGroupResource) Less(i, j int) bool {
	ret := strings.Compare(s[i].APIResource.Name, s[j].APIResource.Name)
	if ret > 0 {
		return false
	} else if ret == 0 {
		return strings.Compare(s[i].APIGroup, s[j].APIGroup) < 0
	}
	return true
}
