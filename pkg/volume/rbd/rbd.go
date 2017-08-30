/*
Copyright 2014 The Kubernetes Authors.

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

package rbd

import (
	"fmt"
	"os"
	dstrings "strings"
	"time"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/uuid"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/util/mount"
	"k8s.io/kubernetes/pkg/util/strings"
	"k8s.io/kubernetes/pkg/volume"
	volutil "k8s.io/kubernetes/pkg/volume/util"
	"k8s.io/kubernetes/pkg/volume/util/volumehelper"
)

var (
	supportedFeatures = sets.NewString("layering")
)

// This is the primary entrypoint for volume plugins.
func ProbeVolumePlugins() []volume.VolumePlugin {
	return []volume.VolumePlugin{&rbdPlugin{nil, nil, nil}}
}

// rbdPlugin implements Volume.VolumePlugin.
type rbdPlugin struct {
	host    volume.VolumeHost
	exec    mount.Exec
	mounter *mount.SafeFormatAndMount
}

var _ volume.VolumePlugin = &rbdPlugin{}
var _ volume.PersistentVolumePlugin = &rbdPlugin{}
var _ volume.DeletableVolumePlugin = &rbdPlugin{}
var _ volume.ProvisionableVolumePlugin = &rbdPlugin{}
var _ volume.AttachableVolumePlugin = &rbdPlugin{}

const (
	rbdPluginName                  = "kubernetes.io/rbd"
	secretKeyName                  = "key" // key name used in secret
	rbdImageFormat1                = "1"
	rbdImageFormat2                = "2"
	rbdDefaultAdminId              = "admin"
	rbdDefaultAdminSecretNamespace = "default"
	rbdDefaultPool                 = "rbd"
	rbdDefaultUserId               = rbdDefaultAdminId
)

func getPath(uid types.UID, volName string, host volume.VolumeHost) string {
	return host.GetPodVolumeDir(uid, strings.EscapeQualifiedNameForDisk(rbdPluginName), volName)
}

func (plugin *rbdPlugin) Init(host volume.VolumeHost) error {
	plugin.host = host
	plugin.exec = host.GetExec(plugin.GetPluginName())
	plugin.mounter = volumehelper.NewSafeFormatAndMountFromHost(plugin.GetPluginName(), plugin.host)
	return nil
}

func (plugin *rbdPlugin) GetPluginName() string {
	return rbdPluginName
}

func (plugin *rbdPlugin) GetVolumeName(spec *volume.Spec) (string, error) {
	volumeSource, _, err := getVolumeSource(spec)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"%v:%v",
		volumeSource.CephMonitors,
		volumeSource.RBDImage), nil
}

func (plugin *rbdPlugin) CanSupport(spec *volume.Spec) bool {
	if (spec.Volume != nil && spec.Volume.RBD == nil) || (spec.PersistentVolume != nil && spec.PersistentVolume.Spec.RBD == nil) {
		return false
	}

	return true
}

func (plugin *rbdPlugin) RequiresRemount() bool {
	return false
}

func (plugin *rbdPlugin) SupportsMountOption() bool {
	return true
}

func (plugin *rbdPlugin) SupportsBulkVolumeVerification() bool {
	return false
}

func (plugin *rbdPlugin) GetAccessModes() []v1.PersistentVolumeAccessMode {
	return []v1.PersistentVolumeAccessMode{
		v1.ReadWriteOnce,
		v1.ReadOnlyMany,
	}
}

func (plugin *rbdPlugin) createMounterFromVolumeSpec(spec *volume.Spec) (*rbdMounter, error) {
	var secret string
	var err error
	source, _, err := getVolumeSource(spec)
	if err != nil {
		return nil, err
	}
	if source.SecretRef != nil {
		if secret, err = parsePVSecret(spec.Namespace, source.SecretRef.Name, plugin.host.GetKubeClient()); err != nil {
			glog.Errorf("Couldn't get secret from %v/%v", spec.Namespace, source.SecretRef)
			return nil, err
		}
	}

	id := source.RadosUser
	keyring := source.Keyring

	return &rbdMounter{
		rbd:     newRBD("", spec.Name(), source.RBDImage, source.RBDPool, spec.ReadOnly, plugin, &RBDUtil{}),
		Mon:     source.CephMonitors,
		Id:      id,
		Keyring: keyring,
		Secret:  secret,
		fsType:  source.FSType,
	}, nil
}

func (plugin *rbdPlugin) NewAttacher() (volume.Attacher, error) {
	return plugin.newAttacherInternal(&RBDUtil{})
}

func (plugin *rbdPlugin) newAttacherInternal(manager rbdUtil) (volume.Attacher, error) {
	return &rbdAttacher{
		plugin:  plugin,
		manager: manager,
	}, nil
}

func (plugin *rbdPlugin) GetDeviceMountRefs(deviceMountPath string) ([]string, error) {
	mounter := plugin.host.GetMounter(plugin.GetPluginName())
	return mount.GetMountRefs(mounter, deviceMountPath)
}

func (plugin *rbdPlugin) NewDetacher() (volume.Detacher, error) {
	return plugin.newDetacherInternal(&RBDUtil{})
}

func (plugin *rbdPlugin) newDetacherInternal(manager rbdUtil) (volume.Detacher, error) {
	return &rbdDetacher{
		plugin:  plugin,
		manager: manager,
	}, nil
}

func (plugin *rbdPlugin) NewMounter(spec *volume.Spec, pod *v1.Pod, _ volume.VolumeOptions) (volume.Mounter, error) {
	var secret string
	var err error
	source, _, err := getVolumeSource(spec)
	if err != nil {
		return nil, err
	}

	if source.SecretRef != nil {
		if secret, err = parsePodSecret(pod, source.SecretRef.Name, plugin.host.GetKubeClient()); err != nil {
			glog.Errorf("Couldn't get secret from %v/%v", pod.Namespace, source.SecretRef)
			return nil, err
		}
	}

	// Inject real implementations here, test through the internal function.
	return plugin.newMounterInternal(spec, pod.UID, &RBDUtil{}, secret)
}

func (plugin *rbdPlugin) newMounterInternal(spec *volume.Spec, podUID types.UID, manager rbdUtil, secret string) (volume.Mounter, error) {
	source, readOnly, err := getVolumeSource(spec)
	if err != nil {
		return nil, err
	}
	pool := source.RBDPool
	id := source.RadosUser
	keyring := source.Keyring

	return &rbdMounter{
		rbd:          newRBD(podUID, spec.Name(), source.RBDImage, pool, readOnly, plugin, manager),
		Mon:          source.CephMonitors,
		Id:           id,
		Keyring:      keyring,
		Secret:       secret,
		fsType:       source.FSType,
		mountOptions: volume.MountOptionFromSpec(spec),
	}, nil
}

func (plugin *rbdPlugin) NewUnmounter(volName string, podUID types.UID) (volume.Unmounter, error) {
	// Inject real implementations here, test through the internal function.
	return plugin.newUnmounterInternal(volName, podUID, &RBDUtil{})
}

func (plugin *rbdPlugin) newUnmounterInternal(volName string, podUID types.UID, manager rbdUtil) (volume.Unmounter, error) {
	return &rbdUnmounter{
		rbdMounter: &rbdMounter{
			rbd: newRBD(podUID, volName, "", "", false, plugin, manager),
			Mon: make([]string, 0),
		},
	}, nil
}

func (plugin *rbdPlugin) ConstructVolumeSpec(volumeName, mountPath string) (*volume.Spec, error) {
	rbdVolume := &v1.Volume{
		Name: volumeName,
		VolumeSource: v1.VolumeSource{
			RBD: &v1.RBDVolumeSource{
				CephMonitors: []string{},
			},
		},
	}
	return volume.NewSpecFromVolume(rbdVolume), nil
}

func (plugin *rbdPlugin) NewDeleter(spec *volume.Spec) (volume.Deleter, error) {
	if spec.PersistentVolume != nil && spec.PersistentVolume.Spec.RBD == nil {
		return nil, fmt.Errorf("spec.PersistentVolumeSource.Spec.RBD is nil")
	}
	class, err := volutil.GetClassForVolume(plugin.host.GetKubeClient(), spec.PersistentVolume)
	if err != nil {
		return nil, err
	}
	adminSecretName := ""
	adminSecretNamespace := rbdDefaultAdminSecretNamespace
	admin := ""

	for k, v := range class.Parameters {
		switch dstrings.ToLower(k) {
		case "adminid":
			admin = v
		case "adminsecretname":
			adminSecretName = v
		case "adminsecretnamespace":
			adminSecretNamespace = v
		}
	}

	if admin == "" {
		admin = rbdDefaultAdminId
	}
	secret, err := parsePVSecret(adminSecretNamespace, adminSecretName, plugin.host.GetKubeClient())
	if err != nil {
		return nil, fmt.Errorf("failed to get admin secret from [%q/%q]: %v", adminSecretNamespace, adminSecretName, err)
	}
	return plugin.newDeleterInternal(spec, admin, secret, &RBDUtil{})
}

func (plugin *rbdPlugin) newDeleterInternal(spec *volume.Spec, admin, secret string, manager rbdUtil) (volume.Deleter, error) {
	return &rbdVolumeDeleter{
		rbdMounter: &rbdMounter{
			rbd:         newRBD("", spec.Name(), spec.PersistentVolume.Spec.RBD.RBDImage, spec.PersistentVolume.Spec.RBD.RBDPool, false, plugin, manager),
			Mon:         spec.PersistentVolume.Spec.RBD.CephMonitors,
			adminId:     admin,
			adminSecret: secret,
		}}, nil
}

func (plugin *rbdPlugin) NewProvisioner(options volume.VolumeOptions) (volume.Provisioner, error) {
	return plugin.newProvisionerInternal(options, &RBDUtil{})
}

func (plugin *rbdPlugin) newProvisionerInternal(options volume.VolumeOptions, manager rbdUtil) (volume.Provisioner, error) {
	return &rbdVolumeProvisioner{
		rbdMounter: &rbdMounter{
			rbd: newRBD("", "", "", "", false, plugin, manager),
		},
		options: options,
	}, nil
}

// rbdVolumeProvisioner implements volume.Provisioner interface.
type rbdVolumeProvisioner struct {
	*rbdMounter
	options volume.VolumeOptions
}

var _ volume.Provisioner = &rbdVolumeProvisioner{}

func (r *rbdVolumeProvisioner) Provision() (*v1.PersistentVolume, error) {
	if !volume.AccessModesContainedInAll(r.plugin.GetAccessModes(), r.options.PVC.Spec.AccessModes) {
		return nil, fmt.Errorf("invalid AccessModes %v: only AccessModes %v are supported", r.options.PVC.Spec.AccessModes, r.plugin.GetAccessModes())
	}

	if r.options.PVC.Spec.Selector != nil {
		return nil, fmt.Errorf("claim Selector is not supported")
	}
	var err error
	adminSecretName := ""
	adminSecretNamespace := rbdDefaultAdminSecretNamespace
	secretName := ""
	secret := ""
	imageFormat := rbdImageFormat1
	fstype := ""

	for k, v := range r.options.Parameters {
		switch dstrings.ToLower(k) {
		case "monitors":
			arr := dstrings.Split(v, ",")
			for _, m := range arr {
				r.Mon = append(r.Mon, m)
			}
		case "adminid":
			r.adminId = v
		case "adminsecretname":
			adminSecretName = v
		case "adminsecretnamespace":
			adminSecretNamespace = v
		case "userid":
			r.Id = v
		case "pool":
			r.Pool = v
		case "usersecretname":
			secretName = v
		case "imageformat":
			imageFormat = v
		case "imagefeatures":
			arr := dstrings.Split(v, ",")
			for _, f := range arr {
				if !supportedFeatures.Has(f) {
					return nil, fmt.Errorf("invalid feature %q for volume plugin %s, supported features are: %v", f, r.plugin.GetPluginName(), supportedFeatures)
				} else {
					r.imageFeatures = append(r.imageFeatures, f)
				}
			}
		case volume.VolumeParameterFSType:
			fstype = v
		default:
			return nil, fmt.Errorf("invalid option %q for volume plugin %s", k, r.plugin.GetPluginName())
		}
	}
	// sanity check
	if imageFormat != rbdImageFormat1 && imageFormat != rbdImageFormat2 {
		return nil, fmt.Errorf("invalid ceph imageformat %s, expecting %s or %s",
			imageFormat, rbdImageFormat1, rbdImageFormat2)
	}
	r.imageFormat = imageFormat
	if adminSecretName == "" {
		return nil, fmt.Errorf("missing Ceph admin secret name")
	}
	if secret, err = parsePVSecret(adminSecretNamespace, adminSecretName, r.plugin.host.GetKubeClient()); err != nil {
		return nil, fmt.Errorf("failed to get admin secret from [%q/%q]: %v", adminSecretNamespace, adminSecretName, err)
	}
	r.adminSecret = secret
	if len(r.Mon) < 1 {
		return nil, fmt.Errorf("missing Ceph monitors")
	}
	if secretName == "" {
		return nil, fmt.Errorf("missing user secret name")
	}
	if r.adminId == "" {
		r.adminId = rbdDefaultAdminId
	}
	if r.Pool == "" {
		r.Pool = rbdDefaultPool
	}
	if r.Id == "" {
		r.Id = r.adminId
	}

	// create random image name
	image := fmt.Sprintf("kubernetes-dynamic-pvc-%s", uuid.NewUUID())
	r.rbdMounter.Image = image
	rbd, sizeMB, err := r.manager.CreateImage(r)
	if err != nil {
		glog.Errorf("rbd: create volume failed, err: %v", err)
		return nil, err
	}
	glog.Infof("successfully created rbd image %q", image)
	pv := new(v1.PersistentVolume)
	metav1.SetMetaDataAnnotation(&pv.ObjectMeta, volumehelper.VolumeDynamicallyCreatedByKey, "rbd-dynamic-provisioner")
	rbd.SecretRef = new(v1.LocalObjectReference)
	rbd.SecretRef.Name = secretName
	rbd.RadosUser = r.Id
	rbd.FSType = fstype
	pv.Spec.PersistentVolumeSource.RBD = rbd
	pv.Spec.PersistentVolumeReclaimPolicy = r.options.PersistentVolumeReclaimPolicy
	pv.Spec.AccessModes = r.options.PVC.Spec.AccessModes
	if len(pv.Spec.AccessModes) == 0 {
		pv.Spec.AccessModes = r.plugin.GetAccessModes()
	}
	pv.Spec.Capacity = v1.ResourceList{
		v1.ResourceName(v1.ResourceStorage): resource.MustParse(fmt.Sprintf("%dMi", sizeMB)),
	}
	return pv, nil
}

// rbdVolumeDeleter implements volume.Deleter interface.
type rbdVolumeDeleter struct {
	*rbdMounter
}

var _ volume.Deleter = &rbdVolumeDeleter{}

func (r *rbdVolumeDeleter) GetPath() string {
	return getPath(r.podUID, r.volName, r.plugin.host)
}

func (r *rbdVolumeDeleter) Delete() error {
	return r.manager.DeleteImage(r)
}

// rbd implmenets volume.Volume interface.
// It's embedded in Mounter/Unmounter/Deleter.
type rbd struct {
	volName  string
	podUID   types.UID
	Pool     string
	Image    string
	ReadOnly bool
	plugin   *rbdPlugin
	mounter  *mount.SafeFormatAndMount
	exec     mount.Exec
	// Utility interface that provides API calls to the provider to attach/detach disks.
	manager rbdUtil
	volume.MetricsProvider
}

var _ volume.Volume = &rbd{}

func (rbd *rbd) GetPath() string {
	// safe to use PodVolumeDir now: volume teardown occurs before pod is cleaned up
	return getPath(rbd.podUID, rbd.volName, rbd.plugin.host)
}

// newRBD creates a new rbd.
func newRBD(podUID types.UID, volName string, image string, pool string, readOnly bool, plugin *rbdPlugin, manager rbdUtil) *rbd {
	return &rbd{
		podUID:          podUID,
		volName:         volName,
		Image:           image,
		Pool:            pool,
		ReadOnly:        readOnly,
		plugin:          plugin,
		mounter:         plugin.mounter,
		exec:            plugin.exec,
		manager:         manager,
		MetricsProvider: volume.NewMetricsStatFS(getPath(podUID, volName, plugin.host)),
	}
}

// rbdAttacher implements volume.Attacher interface.
type rbdAttacher struct {
	plugin  *rbdPlugin
	manager rbdUtil
}

var _ volume.Attacher = &rbdAttacher{}

// Attach implements Attacher.Attach method. It requires a exclusive lock of
// image for given node from ceph cluster.
// It's called by kube-controller-manager, if kubelet's --enable-controller-attach-detach flag is true.
// Otherwise, it's called by kubelet itself.
func (attacher *rbdAttacher) Attach(spec *volume.Spec, nodeName types.NodeName) (string, error) {
	glog.V(4).Infof("rbd: attaching %s onto %s", spec.Name(), nodeName)
	mounter, err := attacher.plugin.createMounterFromVolumeSpec(spec)
	if err != nil {
		return "", err
	}
	err = attacher.manager.Fencing(*mounter, string(nodeName))
	if err != nil {
		return "", err
	}
	glog.V(3).Infof("rbd: successfully attach %s/%s (%s) onto %s", mounter.Pool, mounter.Image, spec.Name(), nodeName)
	return fmt.Sprintf("/dev/%s/%s", mounter.Pool, mounter.Image), nil
}

func (attacher *rbdAttacher) VolumesAreAttached(specs []*volume.Spec, nodeName types.NodeName) (map[*volume.Spec]bool, error) {
	volumesAttachedCheck := make(map[*volume.Spec]bool)
	for _, spec := range specs {
		mounter, err := attacher.plugin.createMounterFromVolumeSpec(spec)
		if err != nil {
			glog.Warningf("failed to parse volume spec")
			continue
		}
		isLocked, _ := attacher.manager.IsLocked(*mounter, string(nodeName))
		volumesAttachedCheck[spec] = isLocked
	}
	return volumesAttachedCheck, nil
}

func (attacher *rbdAttacher) WaitForAttach(spec *volume.Spec, devicePath string, pod *v1.Pod, timeout time.Duration) (string, error) {
	glog.V(4).Infof("rbd: waiting for attach %s (devicePath: %s)", spec.Name(), devicePath)
	mounter, err := attacher.plugin.createMounterFromVolumeSpec(spec)
	if err != nil {
		glog.Warningf("failed to parse volume spec: %v", spec)
		return "", err
	}
	realDevicePath, err := attacher.manager.AttachDisk(*mounter)
	if err != nil {
		return "", err
	}
	glog.V(3).Infof("rbd: successfully wait for attach image %s/%s (spec: %s) at %s", mounter.Pool, mounter.Image, spec.Name(), realDevicePath)
	return realDevicePath, nil
}

func (attacher *rbdAttacher) GetDeviceMountPath(spec *volume.Spec) (string, error) {
	source, _, err := getVolumeSource(spec)
	if err != nil {
		return "", err
	}
	return makePDNameInternal(attacher.plugin.host, source.RBDPool, source.RBDImage), nil
}

func (attacher *rbdAttacher) MountDevice(spec *volume.Spec, devicePath string, deviceMountPath string) error {
	glog.V(4).Infof("rbd: mouting device %s to %s", devicePath, deviceMountPath)
	mounter, err := attacher.plugin.createMounterFromVolumeSpec(spec)
	if err != nil {
		glog.Warningf("failed to parse volume spec")
		return err
	}
	notMnt, err := mounter.mounter.IsLikelyNotMountPoint(deviceMountPath)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(deviceMountPath, 0750); err != nil {
				return err
			}
			notMnt = true
		} else {
			return err
		}
	}
	if !notMnt {
		return nil
	}
	options := []string{}
	if mounter.rbd.GetAttributes().ReadOnly {
		options = append(options, "ro")
	}
	err = mounter.mounter.FormatAndMount(devicePath, deviceMountPath, mounter.fsType, options)
	if err != nil {
		return fmt.Errorf("rbd: failed to mount device %s [%s] to %s, error %v", devicePath, mounter.fsType, deviceMountPath, err)
	}
	glog.V(3).Infof("rbd: successfully mount device %s (pool: %s, image: %s, fstype: %s) at %s", devicePath, mounter.Pool, mounter.Image, mounter.fsType, deviceMountPath)
	return nil
}

// rbdDetacher implements volume.Detacher interface.
type rbdDetacher struct {
	plugin  *rbdPlugin
	manager rbdUtil
}

var _ volume.Detacher = &rbdDetacher{}

// UnmountDevice unmounts the global mount of the RBD image. This is called
// once all bind mounts have been unmounted.
func (detacher *rbdDetacher) UnmountDevice(deviceMountPath string) error {
	glog.V(4).Infof("rbd: unmouting device mountpoint %s", deviceMountPath)
	devicePath, cnt, err := mount.GetDeviceNameFromMount(detacher.plugin.mounter, deviceMountPath)
	if err != nil {
		return err
	}
	if cnt > 1 {
		return fmt.Errorf("rbd: more than 1 reference counts at %s", deviceMountPath)
	}
	err = volutil.UnmountPath(deviceMountPath, detacher.plugin.mounter)
	if err != nil {
		return err
	}
	glog.V(3).Infof("rbd: successfully umount device mountpoint %s", deviceMountPath)
	if len(devicePath) > 0 && cnt <= 1 {
		// If device exists and no other references, detach it from the machine.
		// TODO Detach device from the machine in Detacher.WaitForDetach once volumemanager uses it.
		glog.V(4).Infof("rbd: detaching device %s", devicePath)
		err = detacher.manager.DetachDisk(detacher.plugin, devicePath)
		if err != nil {
			return err
		}
		glog.V(3).Infof("rbd: successfully detach device %s", devicePath)
	}
	return nil
}

// Detach checks if the specified volume is already attached to the specified
// node. If the volume is not attached, it succeeds (returns nil). If it is
// attached, Detach tries to detach it.
// It's called by kube-controller-manager, if kubelet's --enable-controller-attach-detach flag is true.
// Otherwise, it's called by kubelet itself.
func (detacher *rbdDetacher) Detach(spec *volume.Spec, deviceName string, nodeName types.NodeName) error {
	glog.V(4).Infof("rbd: detaching %s from %s", deviceName, nodeName)
	mounter, err := detacher.plugin.createMounterFromVolumeSpec(spec)
	if err != nil {
		glog.Warningf("rbd: failed to create rbd mounter from spec %v: %v", spec, err)
		return err
	}
	glog.V(3).Infof("rbd: successfully detach %s/%s (%s) from %s", mounter.Pool, mounter.Image, spec.Name(), nodeName)
	return detacher.manager.Defencing(*mounter, string(nodeName))
}

// rbdMounter implements volume.Mounter interface.
// It contains information which need to be persisted in whole life cycle of PV
// on the node. It is persisted at the very beginning in the pod mount point
// directory.
// Note: Capitalized field names of this struct determines the information persisted on the disk, DO NOT change them.
type rbdMounter struct {
	*rbd
	// capitalized so they can be exported in persistRBD()
	Mon           []string
	Id            string
	Keyring       string
	Secret        string
	fsType        string
	adminSecret   string
	adminId       string
	mountOptions  []string
	imageFormat   string
	imageFeatures []string
}

var _ volume.Mounter = &rbdMounter{}

func (b *rbd) GetAttributes() volume.Attributes {
	return volume.Attributes{
		ReadOnly:        b.ReadOnly,
		Managed:         !b.ReadOnly,
		SupportsSELinux: true,
	}
}

// Checks prior to mount operations to verify that the required components (binaries, etc.)
// to mount the volume are available on the underlying node.
// If not, it returns an error
func (b *rbdMounter) CanMount() error {
	return nil
}

func (b *rbdMounter) SetUp(fsGroup *int64) error {
	return b.SetUpAt(b.GetPath(), fsGroup)
}

func (b *rbdMounter) SetUpAt(dir string, fsGroup *int64) error {
	// diskSetUp checks mountpoints and prevent repeated calls
	glog.V(4).Infof("rbd: attempting to setup at %s", dir)
	err := diskSetUp(b.manager, *b, dir, b.mounter, fsGroup)
	if err != nil {
		glog.Errorf("rbd: failed to setup at %s %v", dir, err)
	}
	glog.V(3).Infof("rbd: successfully setup at %s", dir)
	return err
}

// rbdUnmounter implements volume.Unmounter interface.
type rbdUnmounter struct {
	*rbdMounter
}

var _ volume.Unmounter = &rbdUnmounter{}

// Unmounts the bind mount, and detaches the disk only if the disk
// resource was the last reference to that disk on the kubelet.
func (c *rbdUnmounter) TearDown() error {
	return c.TearDownAt(c.GetPath())
}

func (c *rbdUnmounter) TearDownAt(dir string) error {
	glog.V(4).Infof("rbd: attempting to teardown at %s", dir)
	if pathExists, pathErr := volutil.PathExists(dir); pathErr != nil {
		return fmt.Errorf("Error checking if path exists: %v", pathErr)
	} else if !pathExists {
		glog.Warningf("Warning: Unmount skipped because path does not exist: %v", dir)
		return nil
	}
	err := diskTearDown(c.manager, *c, dir, c.mounter)
	if err != nil {
		return err
	}
	glog.V(3).Infof("rbd: successfully teardown at %s", dir)
	return nil
}

func getVolumeSource(
	spec *volume.Spec) (*v1.RBDVolumeSource, bool, error) {
	if spec.Volume != nil && spec.Volume.RBD != nil {
		return spec.Volume.RBD, spec.Volume.RBD.ReadOnly, nil
	} else if spec.PersistentVolume != nil &&
		spec.PersistentVolume.Spec.RBD != nil {
		return spec.PersistentVolume.Spec.RBD, spec.ReadOnly, nil
	}

	return nil, false, fmt.Errorf("Spec does not reference a RBD volume type")
}

func parsePodSecret(pod *v1.Pod, secretName string, kubeClient clientset.Interface) (string, error) {
	secret, err := volutil.GetSecretForPod(pod, secretName, kubeClient)
	if err != nil {
		glog.Errorf("failed to get secret from [%q/%q]: %v", pod.Namespace, secretName, err)
		return "", fmt.Errorf("failed to get secret from [%q/%q]: %v", pod.Namespace, secretName, err)
	}
	return parseSecretMap(secret)
}

func parsePVSecret(namespace, secretName string, kubeClient clientset.Interface) (string, error) {
	secret, err := volutil.GetSecretForPV(namespace, secretName, rbdPluginName, kubeClient)
	if err != nil {
		glog.Errorf("failed to get secret from [%q/%q]: %v", namespace, secretName, err)
		return "", fmt.Errorf("failed to get secret from [%q/%q]: %v", namespace, secretName, err)
	}
	return parseSecretMap(secret)
}

// parseSecretMap locates the secret by key name.
func parseSecretMap(secretMap map[string]string) (string, error) {
	if len(secretMap) == 0 {
		return "", fmt.Errorf("empty secret map")
	}
	secret := ""
	for k, v := range secretMap {
		if k == secretKeyName {
			return v, nil
		}
		secret = v
	}
	// If not found, the last secret in the map wins as done before
	return secret, nil
}
