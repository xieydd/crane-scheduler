package annotator

import (
	"context"
	"fmt"
	"strconv"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	schedulingapi "git.woa.com/crane/api/scheduling/v1alpha1"

	"github.com/gocrane/crane-scheduler/pkg/controller/metrics"
	"github.com/gocrane/crane-scheduler/pkg/known"
	"github.com/gocrane/crane-scheduler/pkg/plugins/apis/policy"
	"github.com/gocrane/crane-scheduler/pkg/utils"
)

const (
	DefaultBackOff = 10 * time.Second
	MaxBackOff     = 360 * time.Second
)

type nodeController struct {
	*Controller
	queue workqueue.RateLimitingInterface
}

func newNodeController(c *Controller) *nodeController {
	nodeRateLimiter := workqueue.NewItemExponentialFailureRateLimiter(DefaultBackOff,
		MaxBackOff)

	return &nodeController{
		Controller: c,
		queue:      workqueue.NewNamedRateLimitingQueue(nodeRateLimiter, "node_event_queue"),
	}
}

func (n *nodeController) Run() {
	defer n.queue.ShutDown()
	defer runtime.HandleCrash()
	klog.Infof("Start to reconcile node events")

	for n.processNextWorkItem() {
	}
}

func (n *nodeController) processNextWorkItem() bool {
	key, quit := n.queue.Get()
	if quit {
		return false
	}
	defer n.queue.Done(key)

	forget, err := n.syncNode(key.(string))
	if err != nil {
		klog.Warningf("failed to sync this node [%q]: %v", key.(string), err)
	}
	if forget {
		n.queue.Forget(key)
		return true
	}

	n.queue.AddRateLimited(key)
	return true
}

func (n *nodeController) syncNode(key string) (bool, error) {
	startTime := time.Now()
	defer func() {
		klog.Infof("Finished syncing node metric %q (%v)", key, time.Since(startTime))
	}()

	nodeName, metricName, err := splitMetaKeyWithMetricName(key)
	if err != nil {
		return true, fmt.Errorf("invalid resource key: %s", key)
	}

	node, err := n.nodeLister.Get(nodeName)
	if err != nil {
		return true, fmt.Errorf("can not find node[%s]: %v", node, err)
	}

	err = annotateNodeLoad(n.metricClient, n.kubeClient, node, metricName)
	if err != nil {
		return false, fmt.Errorf("can not annotate node[%s]: %v", node.Name, err)
	}

	err = annotateNodeHotValue(n.kubeClient, n.bindingRecords, node, n.policy)
	if err != nil {
		return false, err
	}

	return true, nil
}

func annotateNodeLoad(metricClient metrics.MetricClient, kubeClient clientset.Interface, node *v1.Node, key string) error {
	value, _, err := metricClient.QueryNodeMetricLatest(key, node)
	if err != nil || len(value) == 0 {
		return fmt.Errorf("failed to get data %s{%s=%s}: %v", key, node.Name, value, err)
	}

	return patchNodeAnnotation(kubeClient, node, key, value)
}

func annotateNodeHotValue(kubeClient clientset.Interface, br *BindingRecords, node *v1.Node, policy policy.DynamicSchedulerPolicy) error {
	var value int

	for _, p := range policy.Spec.HotValue {
		value += br.GetLastNodeBindingCount(node.Name, p.TimeRange.Duration) / p.Count
	}

	return patchNodeAnnotation(kubeClient, node, known.NodeHotValueKey, strconv.Itoa(value))
}

func patchNodeAnnotation(kubeClient clientset.Interface, node *v1.Node, key, value string) error {
	annotation := node.GetAnnotations()
	if annotation == nil {
		annotation = map[string]string{}
	}

	operator := "add"
	_, exist := annotation[key]
	if exist {
		operator = "replace"
	}

	patchPathKey := utils.BuildCranePatchAnnotationKey(schedulingapi.AnnotationPrefixSchedulingBalanceLoad, key)
	patchAnnotationTemplate :=
		`[{
		"op": "%s",
		"path": "/metadata/annotations/%s",
		"value": "%s"
	}]`

	patchData := fmt.Sprintf(patchAnnotationTemplate, operator, patchPathKey, value+","+utils.GetLocalTime())

	_, err := kubeClient.CoreV1().Nodes().Patch(context.TODO(), node.Name, types.JSONPatchType, []byte(patchData), metav1.PatchOptions{})
	return err
}

func (n *nodeController) CreateMetricSyncTicker(stopCh <-chan struct{}) {
	enqueueFunc := func(policy policy.SyncPolicy) {
		cnrps, err := n.cnrpLister.List(labels.Everything())
		if err != nil {
			panic(fmt.Errorf("failed to list cnrps: %v", err))
		}
		selectedNodes := sets.NewString()
		// todo: also consider the nrp crd when it uses nrp crd mode to each node
		for _, cnrp := range cnrps {
			klog.V(6).Infof("Get cnrp: %v, policy: %+v, nodeSelector: %v", klog.KObj(cnrp), policy, cnrp.Spec.NodeSelector.String())
			cnrpSelector, err := metav1.LabelSelectorAsSelector(&cnrp.Spec.NodeSelector)
			if err != nil {
				klog.Errorf("Failed to convert label selector for cnrp: %v, policy: %v, err: %v", klog.KObj(cnrp), policy, err)
				continue
			}
			nodes, err := n.nodeLister.List(cnrpSelector)
			if err != nil {
				klog.Errorf("Failed to list nodes for cnrp: %v, policy: %v, err: %v", klog.KObj(cnrp), policy, err)
				continue
			}
			for _, node := range nodes {
				selectedNodes.Insert(node.Name)
			}
		}
		klog.V(6).Infof("Get nodes need to update load. policy: %v, cnrp nums: %v, nodes: %v", policy, len(cnrps), selectedNodes.List())

		for _, node := range selectedNodes.List() {
			n.queue.Add(handlingMetaKeyWithMetricName(node, policy.Name))
		}
	}

	for _, p := range n.policy.Spec.SyncPeriod {
		enqueueFunc(p)
		go func(policy policy.SyncPolicy) {
			ticker := time.NewTicker(policy.Period.Duration)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					enqueueFunc(policy)
				case <-stopCh:
					return
				}
			}
		}(p)
	}
}
