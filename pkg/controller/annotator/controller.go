package annotator

import (
	"fmt"
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	craneschedinformers "git.woa.com/crane/api/pkg/generated/informers/externalversions/scheduling/v1alpha1"
	craneschedlisters "git.woa.com/crane/api/pkg/generated/listers/scheduling/v1alpha1"
	"github.com/gocrane/crane-scheduler/pkg/controller/metrics"
	"github.com/gocrane/crane-scheduler/pkg/plugins/apis/policy"
)

// Controller is Controller for node annotator.
type Controller struct {
	nodeInformer       coreinformers.NodeInformer
	nodeInformerSynced cache.InformerSynced
	nodeLister         corelisters.NodeLister

	eventInformer       coreinformers.EventInformer
	eventInformerSynced cache.InformerSynced
	eventLister         corelisters.EventLister

	cnrpInformer       craneschedinformers.ClusterNodeResourcePolicyInformer
	cnrpInformerSynced cache.InformerSynced
	cnrpLister         craneschedlisters.ClusterNodeResourcePolicyLister

	nrpInformer       craneschedinformers.NodeResourcePolicyInformer
	nrpInformerSynced cache.InformerSynced
	nrpLister         craneschedlisters.NodeResourcePolicyLister

	kubeClient   clientset.Interface
	metricClient metrics.MetricClient

	policy         policy.DynamicSchedulerPolicy
	bindingRecords *BindingRecords
}

// NewController returns a Node Annotator object.
func NewNodeAnnotator(
	nodeInformer coreinformers.NodeInformer,
	eventInformer coreinformers.EventInformer,
	cnrpInformer craneschedinformers.ClusterNodeResourcePolicyInformer,
	nrpInformer craneschedinformers.NodeResourcePolicyInformer,
	kubeClient clientset.Interface,
	metricClient metrics.MetricClient,
	policy policy.DynamicSchedulerPolicy,
	bindingHeapSize int32,
) *Controller {
	return &Controller{
		nodeInformer:        nodeInformer,
		nodeInformerSynced:  nodeInformer.Informer().HasSynced,
		nodeLister:          nodeInformer.Lister(),
		cnrpInformer:        cnrpInformer,
		cnrpInformerSynced:  cnrpInformer.Informer().HasSynced,
		cnrpLister:          cnrpInformer.Lister(),
		nrpInformer:         nrpInformer,
		nrpInformerSynced:   nrpInformer.Informer().HasSynced,
		nrpLister:           nrpInformer.Lister(),
		eventInformer:       eventInformer,
		eventInformerSynced: eventInformer.Informer().HasSynced,
		eventLister:         eventInformer.Lister(),
		kubeClient:          kubeClient,
		metricClient:        metricClient,
		policy:              policy,
		bindingRecords:      NewBindingRecords(bindingHeapSize, getMaxHotVauleTimeRange(policy.Spec.HotValue)),
	}
}

// Run runs node annotator.
func (c *Controller) Run(worker int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()

	eventController := newEventController(c)
	c.eventInformer.Informer().AddEventHandler(eventController.handles())

	nodeController := newNodeController(c)

	cnrpController := newClusterNodeResourcePolicyController(c)
	c.cnrpInformer.Informer().AddEventHandler(cnrpController.handles())

	if !cache.WaitForCacheSync(stopCh, c.nodeInformerSynced, c.eventInformerSynced, c.cnrpInformerSynced) {
		return fmt.Errorf("failed to wait for cache sync for annotator")
	}
	klog.Info("Caches are synced for controller")

	for i := 0; i < worker; i++ {
		go wait.Until(nodeController.Run, time.Second, stopCh)
		go wait.Until(eventController.Run, time.Second, stopCh)
		go wait.Until(cnrpController.Run, time.Second, stopCh)
	}

	go wait.Until(c.bindingRecords.BindingsGC, time.Minute, stopCh)

	nodeController.CreateMetricSyncTicker(stopCh)

	<-stopCh
	return nil
}
