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

package app

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	informers "k8s.io/kubernetes/pkg/client/informers/informers_generated/externalversions"
	coreinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/externalversions/core/v1"
	"k8s.io/kubernetes/plugin/cmd/kube-scheduler/app/options"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"

	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/clientset_generated/clientset"

	clientv1 "k8s.io/api/core/v1"

	"k8s.io/kubernetes/plugin/pkg/scheduler"
	_ "k8s.io/kubernetes/plugin/pkg/scheduler/algorithmprovider"
	schedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api"
	latestschedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api/latest"
	"k8s.io/kubernetes/plugin/pkg/scheduler/factory"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/api/errors"
)

func createRecorder(kubecli *clientset.Clientset, s *options.SchedulerServer) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: v1core.New(kubecli.Core().RESTClient()).Events("")})
	return eventBroadcaster.NewRecorder(api.Scheme, clientv1.EventSource{Component: s.SchedulerName})
}

// TODO: convert scheduler to only use client-go's clientset.
func createClient(s *options.SchedulerServer) (*clientset.Clientset, *kubernetes.Clientset, error) {
	kubeconfig, err := clientcmd.BuildConfigFromFlags(s.Master, s.Kubeconfig)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to build config from flags: %v", err)
	}

	kubeconfig.ContentType = s.ContentType
	// Override kubeconfig qps/burst settings from flags
	kubeconfig.QPS = s.KubeAPIQPS
	kubeconfig.Burst = int(s.KubeAPIBurst)

	cli, err := clientset.NewForConfig(restclient.AddUserAgent(kubeconfig, "leader-election"))
	if err != nil {
		return nil, nil, fmt.Errorf("invalid API configuration: %v", err)
	}
	clientgoCli, err := kubernetes.NewForConfig(restclient.AddUserAgent(kubeconfig, "leader-election"))
	if err != nil {
		return nil, nil, fmt.Errorf("invalid API configuration: %v", err)
	}
	return cli, clientgoCli, nil
}

// CreateScheduler encapsulates the entire creation of a runnable scheduler.
func CreateScheduler(
	s *options.SchedulerServer,
	kubecli *clientset.Clientset,
	sharedInformerFactory informers.SharedInformerFactory,
	podInformer coreinformers.PodInformer,
	recorder record.EventRecorder,
) (*scheduler.Scheduler, error) {
	configurator := factory.NewConfigFactory(
		s.SchedulerName,
		kubecli,
		sharedInformerFactory.Core().V1().Nodes(),
		podInformer,
		sharedInformerFactory.Core().V1().PersistentVolumes(),
		sharedInformerFactory.Core().V1().PersistentVolumeClaims(),
		sharedInformerFactory.Core().V1().ReplicationControllers(),
		sharedInformerFactory.Extensions().V1beta1().ReplicaSets(),
		sharedInformerFactory.Apps().V1beta1().StatefulSets(),
		sharedInformerFactory.Core().V1().Services(),
		s.HardPodAffinitySymmetricWeight,
	)

	// Rebuild the configurator with a default Create(...) method.
	schedConfigurator := &schedulerConfigurator{
		configurator,
		s.PolicyConfigFile,
		s.AlgorithmProvider,
		s.PolicyConfigMapName,
		s.PolicyConfigMapNamespace,
		s.UseLegacyPolicyConfig,
		nil,
	}

	scheduler, err := scheduler.NewFromConfigurator(schedConfigurator, func(cfg *scheduler.Config) {
		cfg.Recorder = recorder
		schedConfigurator.schedulerConfig = cfg
	})

	// Install event handlers for changes to the scheduler's ConfigMap
	if !s.UseLegacyPolicyConfig && len(s.PolicyConfigMapName) != 0 {
		schedConfigurator.SetupPolicyConfigMapEventHandlers(kubecli, sharedInformerFactory)
	}

	return scheduler, err
}

// schedulerConfigurator is an interface wrapper that provides a way to create
// a scheduler from a user provided config file or ConfigMap object.
type schedulerConfigurator struct {
	scheduler.Configurator
	policyFile               string
	algorithmProvider        string
	policyConfigMap          string
	policyConfigMapNamespace string
	useLegacyPolicyConfig    bool
	schedulerConfig          *scheduler.Config
}

// getSchedulerPolicyConfig finds and decodes scheduler's policy config. If no
// such policy is found, it returns nil, nil.
func (sc *schedulerConfigurator) getSchedulerPolicyConfig() (*schedulerapi.Policy, error) {
	var configData []byte
	var policyConfigMapFound bool
	var policy schedulerapi.Policy

	// If not in legacy mode, try to find policy ConfigMap.
	if !sc.useLegacyPolicyConfig && len(sc.policyConfigMap) != 0 {
		namespace := sc.policyConfigMapNamespace
		policyConfigMap, err := sc.GetClient().CoreV1().ConfigMaps(namespace).Get(sc.policyConfigMap, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			glog.Warningf("No policy ConfigMap object with %v name found in namespace %v. Falling back to config file if exists...", sc.policyConfigMap, namespace)
		} else {
			if err != nil {
				return nil, fmt.Errorf("Error getting scheduler policy ConfigMap: %v.", err)
			}
			policyConfigMapFound = true
			if policyConfigMap != nil {
				var configString string
				configString, policyConfigDataFound := policyConfigMap.Data[options.SchedulerPolicyConfigMapKey]
				if !policyConfigDataFound {
					return nil, fmt.Errorf("No element with key = '%v' is found in the ConfigMap 'Data'.", options.SchedulerPolicyConfigMapKey)
				}
				glog.Infof("Scheduler policy ConfigMap: %v", configString)
				configData = []byte(configString)
			}
		}
	}

	// If we are in legacy mode or ConfigMap name is empty, try to use policy
	// config file.
	if sc.useLegacyPolicyConfig || len(sc.policyConfigMap) == 0 || !policyConfigMapFound {
		if _, err := os.Stat(sc.policyFile); err != nil {
			// No config file is found.
			if len(sc.policyFile) != 0 {
				glog.Warningf("No policy config file \"%v\" was found. Using internal config...", sc.policyFile)
			}
			return nil, nil
		}
		var err error
		configData, err = ioutil.ReadFile(sc.policyFile)
		if err != nil {
			return nil, fmt.Errorf("unable to read policy config: %v", err)
		}
		fmt.Printf("configData is %v\n", configData)
	}

	if err := runtime.DecodeInto(latestschedulerapi.Codec, configData, &policy); err != nil {
		return nil, fmt.Errorf("invalid configuration: %v", err)
	}
	return &policy, nil
}

// Create implements the interface for the Configurator, hence it is exported
// even though the struct is not.
func (sc schedulerConfigurator) Create() (*scheduler.Config, error) {
	policy, err := sc.getSchedulerPolicyConfig()
	if err != nil {
		return nil, err
	}
	// If no policy is found, create scheduler from algorithm provider.
	if policy == nil {
		if sc.Configurator != nil {
			return sc.Configurator.CreateFromProvider(sc.algorithmProvider)
		}
		return nil, fmt.Errorf("Configurator was nil")
	}

	return sc.CreateFromConfig(*policy)
}

func (sc *schedulerConfigurator) SetupPolicyConfigMapEventHandlers(client clientset.Interface, informerFactory informers.SharedInformerFactory) {
	// selector targets only the scheduler's policy ConfigMap.
	selector := cache.NewListWatchFromClient(client.CoreV1().RESTClient(), "configmaps", sc.policyConfigMapNamespace, fields.OneTermEqualSelector(api.ObjectNameField, string(sc.policyConfigMap)))

	sharedIndexInformer := informerFactory.InformerFor(&clientv1.ConfigMap{}, func(client clientset.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
		sharedIndexInformer := cache.NewSharedIndexInformer(
			selector,
			&clientv1.ConfigMap{},
			resyncPeriod,
			cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		)
		return sharedIndexInformer
	})

	sharedIndexInformer.AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    sc.addPolicyConfigMap,
			UpdateFunc: sc.updatePolicyConfigMap,
			DeleteFunc: sc.deletePolicyConfigMap,
		},
		0,
	)
}

func (sc *schedulerConfigurator) addPolicyConfigMap(obj interface{}) {
	glog.Info("Scheduler policy config (%v/%v) was added.", sc.policyConfigMapNamespace, sc.policyConfigMap)
	_, ok := obj.(*clientv1.ConfigMap)
	if !ok {
		glog.Errorf("cannot convert to *v1.ConfigMap: %v", obj)
		return
	}
	sc.KillScheduler()
}

func (sc *schedulerConfigurator) updatePolicyConfigMap(oldObj, newObj interface{}) {
	glog.Info("Received an update to the scheduler policy config (%v/%v).", sc.policyConfigMapNamespace, sc.policyConfigMap)
	_, ok := oldObj.(*clientv1.ConfigMap)
	if !ok {
		glog.Errorf("cannot convert oldObj to *v1.ConfigMap: %v", oldObj)
		return
	}
	// We intentionally do not verify the new config and let the scheduler do it
	// after getting restarted. We believe a crash loop in the case of a config
	// error will be noticed better than just logging the error and ignoring the
	// new config.
	// So, go ahead and kill the scheduler to apply the new config.
	sc.KillScheduler()
}

func (sc *schedulerConfigurator) deletePolicyConfigMap(obj interface{}) {
	glog.Infof("Scheduler's policy ConfigMap (%v/%v) is deleted.", sc.policyConfigMapNamespace, sc.policyConfigMap)
	switch t := obj.(type) {
	case *clientv1.ConfigMap: // Nothing is needed. Jump out of the switch.
	case cache.DeletedFinalStateUnknown:
		_, ok := t.Obj.(*clientv1.ConfigMap)
		if !ok {
			glog.Errorf("cannot convert to *v1.ConfigMap: %v", t.Obj)
			return
		}
	default:
		glog.Errorf("cannot convert to *v1.ConfigMap: %v", t)
		return
	}
	sc.KillScheduler()
}

// schedulerKillFunc is a function that kills the scheduler. It is here mainly for testability. Tests set it to a function to perform an action that can be verified in tests instead of the default behavior which causes the scheduler to die.
var SchedulerKillFunc func() = nil

func (sc *schedulerConfigurator) KillScheduler() {
	if SchedulerKillFunc != nil {
		SchedulerKillFunc()
	} else {
		glog.Infof("Scheduler is going to die (and restarted) in order to update its policy.")
		if sc.schedulerConfig != nil {
			close(sc.schedulerConfig.StopEverything)
		}
		// The sleep is only to allow cleanups to happen. The 2 second wait is chosen randomly!
		time.Sleep(2 * time.Second)
		glog.Flush()
		os.Exit(0)
	}
}
