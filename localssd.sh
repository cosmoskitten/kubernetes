#!/bin/bash

#export KUBE_GCE_NODE_IMAGE=ubuntu-gke-1604-xenial-v20170816-1
#export KUBE_GCE_NODE_PROJECT=ubuntu-os-gke-cloud
#export KUBE_NODE_OS_DISTRIBUTION=ubuntu

export NODE_LOCAL_SSDS_EXT="1,nvme,fs;1,scsi,fs;1,nvme,block;1,scsi,block"
#export NODE_LOCAL_SSDS_EXT=""
export NODE_LOCAL_SSDS=1
export NUM_NODES=2
#export KUBE_FEATURE_GATES="PersistentLocalVolumes=true"

cluster/kube-up.sh
#hack/e2e-internal/e2e-up.sh

