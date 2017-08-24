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

package bulk

import (
	"fmt"
	"net/http"
	"time"

	restful "github.com/emicklei/go-restful"
	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	bulkapi "k8s.io/apiserver/pkg/apis/bulk"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/server/mux"
)

// APIManagerFactory constructs instances of APIManager
type APIManagerFactory struct {
	NegotiatedSerializer runtime.NegotiatedSerializer
	ContextMapper        request.RequestContextMapper
	Authorizer           authorizer.Authorizer
	Root                 string
	Delegate             *APIManager
	WebsocketTimeout     time.Duration
	PermissionRecheck    time.Duration
}

// EnabledAPIGroupInfo contains necessary context information for API group.
type EnabledAPIGroupInfo struct {
	Storage      map[string]rest.Storage
	GroupVersion schema.GroupVersion
	Mapper       meta.RESTMapper
	Linker       runtime.SelfLinker
	Serializer   runtime.NegotiatedSerializer
}

// APIManager installs web handlers for Bulk API.
type APIManager struct {
	GroupVersion schema.GroupVersion

	// Available api groups.
	APIGroups map[schema.GroupVersion]*EnabledAPIGroupInfo

	// Map from group name to preferred version.
	PreferredVersion map[string]string

	// Performs authorization / admission for bulk api.
	Authorizer authorizer.Authorizer

	NegotiatedSerializer runtime.NegotiatedSerializer
	Mapper               request.RequestContextMapper

	// Value used to set read & write deadlines on websocket connections.
	WebsocketTimeout  time.Duration
	PermissionRecheck time.Duration
	Root              string
}

// New constructs new instance of *APIManager
func (f APIManagerFactory) New() *APIManager {
	glog.V(7).Infof("Construct new bulk.APIManager from %v", f)
	// TODO: merge NegotiatedSerializer & ContextMapper from .Delegate

	// Merge API groups from delegate
	preferredVersion := make(map[string]string)
	groups := make(map[schema.GroupVersion]*EnabledAPIGroupInfo)
	if f.Delegate != nil {
		for k, v := range f.Delegate.APIGroups {
			glog.V(8).Infof("Reuse %v from delegated bulk.APIManager", k)
			groups[k] = v
		}
		for k, v := range f.Delegate.PreferredVersion {
			preferredVersion[k] = v
		}
	}

	return &APIManager{
		// FIXME: Don't hardcode version
		GroupVersion:         schema.GroupVersion{Version: "v1alpha1", Group: bulkapi.GroupName},
		PreferredVersion:     preferredVersion,
		APIGroups:            groups,
		NegotiatedSerializer: f.NegotiatedSerializer,
		Mapper:               f.ContextMapper,
		Root:                 f.Root,
		Authorizer:           f.Authorizer,
		PermissionRecheck:    f.PermissionRecheck,
		WebsocketTimeout:     f.WebsocketTimeout,
	}
}

func handlerToRouteFunction(h http.Handler) restful.RouteFunction {
	return func(req *restful.Request, resp *restful.Response) {
		h.ServeHTTP(resp.ResponseWriter, req.Request)
	}
}

// Install adds the handlers to the given mux.
func (m *APIManager) Install(c *mux.PathRecorderMux) {
	prefix := fmt.Sprintf("%s/bulk", m.Root)
	c.HandleFunc(prefix+"/watch", watchHTTPHandler{m}.ServeHTTP)
}

// RegisterAPIGroup enables Bulk API for provided group.
func (m *APIManager) RegisterAPIGroup(agv EnabledAPIGroupInfo, preferredVersion bool) error {
	if _, found := m.APIGroups[agv.GroupVersion]; found {
		return fmt.Errorf("group %v already registered", agv)
	}
	if _, found := m.PreferredVersion[agv.GroupVersion.Group]; preferredVersion && found {
		return fmt.Errorf("group %v already has preferred version", agv)
	}

	glog.V(7).Infof("Register %v in bulk.APIManager", agv)
	m.APIGroups[agv.GroupVersion] = &agv
	if preferredVersion {
		m.PreferredVersion[agv.GroupVersion.Group] = agv.GroupVersion.Version
	}
	return nil
}
