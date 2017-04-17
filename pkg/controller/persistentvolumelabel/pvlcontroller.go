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

package cloud

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"

	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/aws"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/gce"
	"k8s.io/kubernetes/pkg/controller"
	kubeletapis "k8s.io/kubernetes/pkg/kubelet/apis"
	vol "k8s.io/kubernetes/pkg/volume"
)

const initializerName = "pvlabel.kubernetes.io"
const initialzerConfigName = "pvlabel-initconfig"

// PersistentVolumeLabelController handles adding labels to persistent volumes when they are created
type PersistentVolumeLabelController struct {
	// Control access to cloud volumes
	mutex            sync.Mutex
	ebsVolumes       aws.Volumes
	gceCloudProvider *gce.GCECloud

	cloud         cloudprovider.Interface
	kubeClient    kubernetes.Interface
	pvlController cache.Controller
	volumeLister  corelisters.PersistentVolumeLister

	syncHandler func(vol *v1.PersistentVolume) error

	// queue is where incoming work is placed to de-dup and to allow "easy" rate limited requeues on errors
	queue workqueue.RateLimitingInterface
}

// NewPersistentVolumeLabelController creates a PersistentVolumeLabelController object
func NewPersistentVolumeLabelController(
	kubeClient kubernetes.Interface,
	cloud cloudprovider.Interface) *PersistentVolumeLabelController {
	var pvlIndexer cache.Indexer

	pvlc := &PersistentVolumeLabelController{
		cloud:      cloud,
		kubeClient: kubeClient,
		queue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "pvLabels"),
	}
	pvlc.syncHandler = pvlc.AddLabels
	pvlIndexer, pvlc.pvlController = cache.NewIndexerInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				options.IncludeUninitialized = true
				return kubeClient.CoreV1().PersistentVolumes().List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				options.IncludeUninitialized = true
				return kubeClient.CoreV1().PersistentVolumes().Watch(options)
			},
		},
		&v1.PersistentVolume{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				key, err := cache.MetaNamespaceKeyFunc(obj)
				if err == nil {
					pvlc.queue.Add(key)
				}
			},
		},
		cache.Indexers{},
	)
	pvlc.volumeLister = corelisters.NewPersistentVolumeLister(pvlIndexer)

	return pvlc
}

// Run starts a controller that adds labels to persistent volumes
func (pvlc *PersistentVolumeLabelController) Run(threadiness int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer pvlc.queue.ShutDown()

	glog.Infof("Starting PersistentVolumeLabelController")
	defer glog.Infof("Shutting down PersistentVolumeLabelController")

	go pvlc.pvlController.Run(stopCh)

	if !controller.WaitForCacheSync("persistent volume label", stopCh, pvlc.pvlController.HasSynced) {
		return
	}

	// start up your worker threads based on threadiness.  Some controllers have multiple kinds of workers
	for i := 0; i < threadiness; i++ {
		// runWorker will loop until "something bad" happens.  The .Until will then rekick the worker
		// after one second
		go wait.Until(pvlc.runWorker, time.Second, stopCh)
	}

	// wait until we're told to stop
	<-stopCh
}

func (pvlc *PersistentVolumeLabelController) runWorker() {
	// hot loop until we're told to stop.  processNextWorkItem will automatically wait until there's work
	// available, so we don't worry about secondary waits
	for pvlc.processNextWorkItem() {
	}
}

// processNextWorkItem deals with one key off the queue.  It returns false when it's time to quit.
func (pvlc *PersistentVolumeLabelController) processNextWorkItem() bool {
	// pull the next work item from queue.  It should be a key we use to lookup something in a cache
	keyObj, quit := pvlc.queue.Get()
	if quit {
		return false
	}
	// you always have to indicate to the queue that you've completed a piece of work
	defer pvlc.queue.Done(keyObj)

	key := keyObj.(string)
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		glog.V(4).Infof("error getting name of volume %q to get volume from informer: %v", key, err)
		return false
	}
	pv, err := pvlc.volumeLister.Get(name)
	if err != nil {
		glog.V(4).Infof("error getting volume %s from informer: %v", name, err)
		return false
	}

	// do your work on the persistent volume.  This method will contains your "do stuff" logic
	err = pvlc.syncHandler(pv)
	if err == nil {
		// if you had no error, tell the queue to stop tracking history for your key.  This will
		// reset things like failure counts for per-item rate limiting
		pvlc.queue.Forget(key)
		return true
	}

	// there was a failure so be sure to report it.  This method allows for pluggable error handling
	// which can be used for things like cluster-monitoring
	utilruntime.HandleError(fmt.Errorf("%v failed with : %v", key, err))

	// since we failed, we should requeue the item to work on later.  This method will add a backoff
	// to avoid hotlooping on particular items (they're probably still not going to work right away)
	// and overall controller protection (everything I've done is broken, this controller needs to
	// calm down or it can starve other useful work) cases.
	pvlc.queue.AddRateLimited(key)

	return true
}

// AddLabels adds appropriate labels to persistent volumes and sets the
// volume as available if successful.
func (pvlc *PersistentVolumeLabelController) AddLabels(volume *v1.PersistentVolume) error {
	var volumeLabels map[string]string

	// Only add labels if in the list of initializers
	if pvlc.shouldLabel(volume) {
		if volume.Spec.AWSElasticBlockStore != nil {
			labels, err := pvlc.findAWSEBSLabels(volume)
			if err != nil {
				return fmt.Errorf("error querying AWS EBS volume %s: %v", volume.Spec.AWSElasticBlockStore.VolumeID, err)
			}
			volumeLabels = labels
		}
		if volume.Spec.GCEPersistentDisk != nil {
			labels, err := pvlc.findGCEPDLabels(volume)
			if err != nil {
				return fmt.Errorf("error querying GCE PD volume %s: %v", volume.Spec.GCEPersistentDisk.PDName, err)
			}
			volumeLabels = labels
		}
		return pvlc.updateVolume(volume.Name, volumeLabels)
	}

	return nil
}

func (pvlc *PersistentVolumeLabelController) findAWSEBSLabels(volume *v1.PersistentVolume) (map[string]string, error) {
	// Ignore any volumes that are being provisioned
	if volume.Spec.AWSElasticBlockStore.VolumeID == vol.ProvisionedVolumeName {
		return nil, nil
	}
	ebsVolumes, err := pvlc.getEBSVolumes()
	if err != nil {
		return nil, err
	}

	// TODO: GetVolumeLabels is actually a method on the Volumes interface
	// If that gets standardized we can refactor to reduce code duplication
	spec := aws.KubernetesVolumeID(volume.Spec.AWSElasticBlockStore.VolumeID)
	labels, err := ebsVolumes.GetVolumeLabels(spec)
	if err != nil {
		return nil, err
	}

	return labels, nil
}

// getEBSVolumes returns the AWS Volumes interface for ebs
func (pvlc *PersistentVolumeLabelController) getEBSVolumes() (aws.Volumes, error) {
	pvlc.mutex.Lock()
	defer pvlc.mutex.Unlock()

	if pvlc.ebsVolumes == nil {
		awsCloudProvider, ok := pvlc.cloud.(*aws.Cloud)
		if !ok {
			// GetCloudProvider has gone very wrong
			return nil, fmt.Errorf("error retrieving AWS cloud provider")
		}
		pvlc.ebsVolumes = awsCloudProvider
	}
	return pvlc.ebsVolumes, nil
}

func (pvlc *PersistentVolumeLabelController) findGCEPDLabels(volume *v1.PersistentVolume) (map[string]string, error) {
	// Ignore any volumes that are being provisioned
	if volume.Spec.GCEPersistentDisk.PDName == vol.ProvisionedVolumeName {
		return nil, nil
	}

	provider, err := pvlc.getGCECloudProvider()
	if err != nil {
		return nil, err
	}

	// If the zone is already labeled, honor the hint
	zone := volume.Labels[kubeletapis.LabelZoneFailureDomain]

	labels, err := provider.GetAutoLabelsForPD(volume.Spec.GCEPersistentDisk.PDName, zone)
	if err != nil {
		return nil, err
	}

	return labels, nil
}

// getGCECloudProvider returns the GCE cloud provider, for use for querying volume labels
func (pvlc *PersistentVolumeLabelController) getGCECloudProvider() (*gce.GCECloud, error) {
	pvlc.mutex.Lock()
	defer pvlc.mutex.Unlock()

	if pvlc.gceCloudProvider == nil {
		gceCloudProvider, ok := pvlc.cloud.(*gce.GCECloud)
		if !ok {
			// GetCloudProvider has gone very wrong
			return nil, fmt.Errorf("error retrieving GCE cloud provider")
		}
		pvlc.gceCloudProvider = gceCloudProvider
	}
	return pvlc.gceCloudProvider, nil
}

func (pvlc *PersistentVolumeLabelController) updateVolume(volName string, volLabels map[string]string) error {
	glog.V(4).Infof("updating PersistentVolume %s", volName)
	curVol, err := pvlc.kubeClient.Core().PersistentVolumes().Get(volName, metav1.GetOptions{IncludeUninitialized: true})
	if err != nil {
		return err
	}
	objCopy := curVol.DeepCopyObject()
	newVolume, ok := objCopy.(*v1.PersistentVolume)
	if !ok {
		return fmt.Errorf("failed to cast copy into persistentvolume object %#v", newVolume)
	}
	if newVolume.Labels == nil {
		newVolume.Labels = make(map[string]string)
	}
	for k, v := range volLabels {
		newVolume.Labels[k] = v
	}
	pvlc.removeInitializer(newVolume)

	oldData, err := json.Marshal(curVol)
	if err != nil {
		return fmt.Errorf("failed to marshal old persistentvolume %#v for persistentvolume %q: %v", curVol, volName, err)
	}

	newData, err := json.Marshal(newVolume)
	if err != nil {
		return fmt.Errorf("failed to marshal new persistentvolume %#v for persistentvolume %q: %v", newVolume, volName, err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, v1.PersistentVolume{})
	if err != nil {
		return fmt.Errorf("failed to create patch for persistentvolume %q: %v", volName, err)
	}

	_, err = pvlc.kubeClient.Core().PersistentVolumes().Patch(string(volName), types.StrategicMergePatchType, patchBytes)
	if err != nil {
		return fmt.Errorf("failed to update PersistentVolume %s: %v", volName, err)
	}
	glog.V(4).Infof("updated PersistentVolume %s", volName)

	return err
}

func (pvlc *PersistentVolumeLabelController) removeInitializer(pv *v1.PersistentVolume) {
	if pv.Initializers == nil {
		return
	}

	var updated []metav1.Initializer
	for _, pending := range pv.Initializers.Pending {
		if pending.Name != initializerName {
			updated = append(updated, pending)
		}
	}
	if len(updated) == len(pv.Initializers.Pending) {
		return
	}
	pv.Initializers.Pending = updated
	if len(updated) == 0 {
		pv.Initializers = nil
	}

	glog.Infof("removed initializer on PersistentVolume %s", pv.Name)

	return
}

func (pvlc *PersistentVolumeLabelController) shouldLabel(pv *v1.PersistentVolume) bool {
	hasInitializer := false

	if pv.Initializers != nil {
		for _, pending := range pv.Initializers.Pending {
			if pending.Name == initializerName {
				hasInitializer = true
				break
			}
		}
	}
	return hasInitializer
}
