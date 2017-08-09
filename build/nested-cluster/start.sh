#!/bin/bash

# Copyright 2017 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

wait_for_docker ()
{
  # Wait for docker.
  until docker version; do sleep 1 ;done
}

start_kubelet ()
{
  # Start the kubelet.
  mkdir -p /etc/kubernetes/manifests
  mkdir -p /etc/srv/kubernetes

  # Change the kubelet to not fail with swap on.
  sed -i 's/ExecStart=\/usr\/bin\/kubelet/ExecStart=\/usr\/bin\/kubelet --fail-swap-on=false/' /etc/systemd/system/kubelet.service.d/kubeadm-10.conf
  systemctl enable kubelet
  systemctl start kubelet
}

start_worker ()
{
  wait_for_docker
  start_kubelet

  # Load docker images
  docker load -i /kube-proxy-amd64.tar

  # Start kubeadm.
  /usr/bin/kubeadm join --token=abcdef.abcdefghijklmnop --skip-preflight-checks 172.18.0.2:6443 2>&1
}

start_master ()
{
  wait_for_docker
  mount --make-rshared /etc/kubernetes
  start_kubelet

  # Load the docker images
  docker load -i /cloud-controller-manager-amd64.tar
  docker load -i /kube-apiserver-amd64.tar
  docker load -i /kube-controller-manager-amd64.tar
  docker load -i /kube-proxy-amd64.tar
  docker load -i /kube-scheduler-amd64.tar

  # Run kubeadm init to config a master.
  /usr/bin/kubeadm init --token=abcdef.abcdefghijklmnop --skip-preflight-checks --kubernetes-version=use-kubeadm-version --pod-network-cidr=192.168.0.0/16 2>&1

  # We need to prevent kube-config from trying to set conntrack values.
  kubectl --kubeconfig=/etc/kubernetes/admin.conf get ds -n kube-system kube-proxy -o json | jq '.spec.template.spec.containers[0].command |= .+ ["--conntrack-max-per-core=0"]' | kubectl --kubeconfig=/etc/kubernetes/admin.conf apply -f -
  # Apply a pod network.
  # Calico is an ip-over-ip overlay network. This saves us from many of the
  # difficulties from configuring an L2 network.
  kubectl --kubeconfig=/etc/kubernetes/admin.conf apply -f http://docs.projectcalico.org/v2.4/getting-started/kubernetes/installation/hosted/kubeadm/1.6/calico.yaml
}

start_cluster ()
{
  wait_for_docker

  # Start some workers.
  echo "Creating testnet"
  docker network create --subnet=172.18.0.0/16 testnet
  docker network ls
  echo "Creating virtual nodes"
  docker load -i /nested-kubelet-bundle.tar
  # Early versions of docker don't allow the bind mount mode to be set, and
  # defaults to rprivate.
  mount --make-rshared /var/kubernetes
  docker run -d --privileged --net testnet --ip 172.18.0.2 -p 443:6443 -v /var/kubernetes:/etc/kubernetes -v /lib/modules:/lib/modules gcr.io/google-containers/nested-kubelet:0.1 master
  docker run -d --privileged --net testnet --ip 172.18.0.3 -v /lib/modules:/lib/modules gcr.io/google-containers/nested-kubelet:0.1 worker
}

# Start docker.
mount --make-rshared /lib/modules/
/bin/dockerd-entrypoint.sh &

systemctl enable kubelet
sed -i 's/ExecStart=\/usr\/bin\/kubelet/ExecStart=\/usr\/bin\/kubelet --fail-swap-on=false/' /etc/systemd/system/kubelet.service.d/kubeadm-10.conf
systemctl start kubelet

# Start a new process to do work.
if [[ $1 == "worker" ]] ; then
  start_worker
elif [[ $1 == "master" ]] ; then
  start_master
else
  start_cluster
fi

