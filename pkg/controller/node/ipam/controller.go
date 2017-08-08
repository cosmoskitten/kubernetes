package ipam

import (
	"fmt"
	"net"
	"sync"
	"time"

	"k8s.io/api/core/v1"
	informers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/gce"
	"k8s.io/kubernetes/pkg/controller/node/ipam/cidrset"
	nodesync "k8s.io/kubernetes/pkg/controller/node/ipam/sync"
	"k8s.io/kubernetes/pkg/controller/node/util"
)

// Config for the IPAM controller.
type Config struct {
	// Resync is the default timeout duration when there are no errors.
	Resync time.Duration
	// MaxBackoff is the maximum timeout when in a error backoff state.
	MaxBackoff time.Duration
	// InitialRetry is the initial retry interval when an error is reported.
	InitialRetry time.Duration
	// Mode to use to synchronize.
	Mode nodesync.NodeSyncMode
}

// Controller is the controller for synchronizing cluster and cloud node
// pod CIDR range assignments.
type Controller struct {
	config  *Config
	adapter *adapter

	lock    sync.Mutex
	syncers map[string]*nodesync.NodeSync

	set *cidrset.CidrSet
}

// NewController returns a new instance of the IPAM controller.
func NewController(
	config *Config,
	kubeClient clientset.Interface,
	cloud cloudprovider.Interface,
	clusterCIDR, serviceCIDR *net.IPNet,
	nodeCIDRMaskSize int) (*Controller, error) {

	gceCloud, ok := cloud.(*gce.GCECloud)
	if !ok {
		return nil, fmt.Errorf("cloud CIDR controller does not support %q provider", cloud.ProviderName())
	}

	c := &Controller{
		config:  config,
		adapter: newAdapter(kubeClient, gceCloud),
		syncers: make(map[string]*nodesync.NodeSync),
		set:     cidrset.NewCIDRSet(clusterCIDR, nodeCIDRMaskSize),
	}

	if err := occupyServiceCIDR(c.set, clusterCIDR, serviceCIDR); err != nil {
		return nil, err
	}

	return c, nil
}

// Init initializes the Controller with the existing list of nodes and
// registers the informers for node chnages. This will start synchronization
// of the node and cloud CIDR range allocations.
func (c *Controller) Init(nodeInformer informers.NodeInformer) error {
	nodes, err := listNodes(c.adapter.k8s)
	if err != nil {
		return err
	}
	for _, node := range nodes.Items {
		func() {
			c.lock.Lock()
			defer c.lock.Unlock()

			// XXX/bowei -- stagger the start of each sync cycle.
			syncer := c.newSyncer(node.Name)
			c.syncers[node.Name] = syncer
			go syncer.Loop(nil)
		}()
	}

	nodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    util.CreateAddNodeHandler(c.onAdd),
		UpdateFunc: util.CreateUpdateNodeHandler(c.onUpdate),
		DeleteFunc: util.CreateDeleteNodeHandler(c.onDelete),
	})

	return nil
}

// occupyServiceCIDR removes the service CIDR range from the cluster CIDR if it
// intersects.
func occupyServiceCIDR(set *cidrset.CidrSet, clusterCIDR, serviceCIDR *net.IPNet) error {
	if clusterCIDR.Contains(serviceCIDR.IP) || serviceCIDR.Contains(clusterCIDR.IP) {
		if err := set.Occupy(serviceCIDR); err != nil {
			return err
		}
	}
	return nil
}

type nodeState struct {
	t Timeout
}

func (ns *nodeState) ReportResult(err error) {
	ns.t.Update(err == nil)
}

func (ns *nodeState) ResyncTimeout() time.Duration {
	return ns.t.Next()
}

func (c *Controller) newSyncer(name string) *nodesync.NodeSync {
	ns := &nodeState{
		Timeout{
			Resync:       c.config.Resync,
			MaxBackoff:   c.config.MaxBackoff,
			InitialRetry: c.config.InitialRetry,
		},
	}
	return nodesync.New(ns, c.adapter, c.adapter, c.config.Mode, name, c.set)
}

func (c *Controller) onAdd(node *v1.Node) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if syncer, ok := c.syncers[node.Name]; !ok {
		syncer = c.newSyncer(node.Name)
		c.syncers[node.Name] = syncer
		go syncer.Loop(nil)
	} else {
		syncer.Update(node)
	}

	return nil
}

func (c *Controller) onUpdate(_, node *v1.Node) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if sync, ok := c.syncers[node.Name]; ok {
		sync.Update(node)
	} else {
		// XXX
	}

	return nil
}

func (c *Controller) onDelete(node *v1.Node) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if syncer, ok := c.syncers[node.Name]; ok {
		syncer.Delete(node)
		delete(c.syncers, node.Name)
	} else {
		// XXX
	}

	return nil
}
