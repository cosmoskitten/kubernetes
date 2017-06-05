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

package plugins

import (
	"fmt"
	"net"

	"github.com/golang/glog"
	restclient "k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/kubectl"
)

type proxier struct {
	clientConfig *restclient.Config
	listener     net.Listener
	ip           string
	port         int
	started      bool
}

func NewProxier(clientConfig *restclient.Config) *proxier {
	return &proxier{
		clientConfig: clientConfig,
		ip:           "127.0.0.1",
	}
}

func (p *proxier) Start() error {
	filter := &kubectl.FilterServer{
		AcceptPaths:   kubectl.MakeRegexpArrayOrDie(kubectl.DefaultPathAcceptRE),
		RejectPaths:   kubectl.MakeRegexpArrayOrDie(kubectl.DefaultPathRejectRE),
		AcceptHosts:   kubectl.MakeRegexpArrayOrDie(kubectl.DefaultHostAcceptRE),
		RejectMethods: kubectl.MakeRegexpArrayOrDie(kubectl.DefaultMethodRejectRE),
	}

	server, err := kubectl.NewProxyServer("", "/", "", filter, p.clientConfig)
	if err != nil {
		return err
	}

	p.listener, err = server.Listen(p.ip, p.port)
	if err != nil {
		return err
	}

	glog.V(8).Infof("Starting to serve API proxy for plugin on: %s", p.listener.Addr())
	go func() {
		err := server.ServeOnListener(p.listener)
		if err != nil {
			glog.Fatal(fmt.Errorf("Unable to start API proxy: %v", err))
		}
	}()

	p.started = true
	return nil
}

func (p *proxier) Stop() error {
	glog.V(8).Infof("Closing API proxy for plugin: %s", p.listener.Addr())
	err := p.listener.Close()
	p.started = err != nil
	return err
}

func (p *proxier) Env() (EnvList, error) {
	if !p.started {
		return EnvList{}, nil
	}

	return EnvList{
		{"KUBECTL_PLUGINS_API_PROXY_ADDR", p.listener.Addr().String()},
	}, nil
}
