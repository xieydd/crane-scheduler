package annotator

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	schedulingapi "git.woa.com/crane/api/scheduling/v1alpha1"
	"github.com/gocrane/crane-scheduler/pkg/utils"
)

type cnrpController struct {
	*Controller
	queue workqueue.RateLimitingInterface
}

func newClusterNodeResourcePolicyController(c *Controller) *cnrpController {
	nodeRateLimiter := workqueue.NewItemExponentialFailureRateLimiter(DefaultBackOff,
		MaxBackOff)

	return &cnrpController{
		Controller: c,
		queue:      workqueue.NewNamedRateLimitingQueue(nodeRateLimiter, "cnrp_queue"),
	}
}

func (n *cnrpController) Run() {
	defer n.queue.ShutDown()
	defer runtime.HandleCrash()
	klog.Infof("Start to reconcile cluster node resource policy")

	for n.processNextWorkItem() {
	}
}

func (n *cnrpController) processNextWorkItem() bool {
	key, quit := n.queue.Get()
	if quit {
		return false
	}
	defer n.queue.Done(key)

	forget, after, err := n.syncCNRP(key.(string))
	if err != nil {
		klog.Warningf("Failed to sync this cnrp [%q]: %v", key.(string), err)
	}
	if forget {
		n.queue.Forget(key)
		return true
	}

	if after > 0 {
		// time tick driven to keep the cnrp consistent with the node selected by the cnrp
		// reconcile to keep the node is final consistent with cnrp template
		n.queue.AddAfter(key, 1*time.Minute)
	} else {
		n.queue.AddRateLimited(key)
	}
	return true
}

func (n *cnrpController) syncCNRP(key string) (bool, time.Duration, error) {
	startTime := time.Now()
	defer func() {
		klog.Infof("Finished syncing cnrp[%s] (%v)", key, time.Since(startTime))
	}()

	_, cnrpName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return true, 0, fmt.Errorf("invalid resource key: %s", key)
	}

	cnrp, err := n.cnrpLister.Get(cnrpName)
	if err != nil {
		if errors.IsNotFound(err) {
			return true, 0, fmt.Errorf("can not find cnrp[%s]: %v, maybe already deleted", cnrpName, err)
		} else {
			return false, 0, fmt.Errorf("failed to get cnrp[%s]: %v", key, err)
		}
	}

	switch cnrp.Spec.ApplyMode {
	case schedulingapi.NodeResourceApplyModeCRD:
		return true, 0, fmt.Errorf("crd is not supported now")
	case schedulingapi.NodeResourceApplyModeAnnotation:
		fallthrough
	default:
		err = n.handleClusterNodeResourcePolicyByAnn(cnrp)
		if err != nil {
			return false, 0, err
		}
	}

	return true, 1 * time.Minute, nil
}

func (n *cnrpController) handleClusterNodeResourcePolicyByAnn(policy *schedulingapi.ClusterNodeResourcePolicy) error {
	selector, err := metav1.LabelSelectorAsSelector(&policy.Spec.NodeSelector)
	if err != nil {
		return err
	}
	nodes, err := n.nodeLister.List(selector)
	if err != nil {
		return err
	}
	strategy := policy.Spec.Template.Spec.ResourceExpansionStrategy
	staticResourceExpansion := policy.Spec.Template.Spec.StaticResourceExpansion
	// no used yet
	autoResourceExpansion := policy.Spec.Template.Spec.AutoResourceExpansion
	if strategy == schedulingapi.ResourceExpansionStrategyTypeTypeStatic && staticResourceExpansion == nil {
		klog.Warningf("auto resource expansion is required: %v", klog.KObj(policy))
		return nil
	}
	if strategy == schedulingapi.ResourceExpansionStrategyTypeTypeAuto && autoResourceExpansion == nil {
		klog.Warningf("auto resource expansion is required: %v", klog.KObj(policy))
		return nil
	}

	if autoResourceExpansion == nil && staticResourceExpansion == nil {
		klog.Warningf("no any resource expansion param: %v", klog.KObj(policy))
		return nil
	}

	if autoResourceExpansion != nil {
		klog.Warningf("auto resource expansion not supported: %v", klog.KObj(policy))
		return nil
	}
	overloadThresholds := policy.Spec.Template.Spec.TargetLoadThreshold

	dswAnns := make(map[string]string)
	for resource, ratio := range staticResourceExpansion.Ratios {
		key := utils.BuildCraneAnnotation(schedulingapi.AnnotationPrefixSchedulingExpansion, resource.String())
		dswAnns[key] = ratio
	}
	if overloadThresholds != nil {
		for resource, percent := range overloadThresholds.Percents {
			key := utils.BuildCraneAnnotation(schedulingapi.AnnotationPrefixSchedulingBalanceTarget, resource.String())
			dswAnns[key] = strconv.FormatInt(percent, 10)
		}
	}
	var errs []error
	for _, node := range nodes {
		anns := node.GetAnnotations()
		aswAnns := utils.GetDswCraneAnnotations(anns)
		if !equality.Semantic.DeepEqual(&aswAnns, &dswAnns) {
			patchData, err := utils.BuildPatchBytes(aswAnns, dswAnns)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			_, err = n.kubeClient.CoreV1().Nodes().Patch(context.TODO(), node.Name, types.JSONPatchType, patchData, metav1.PatchOptions{})
			if err != nil {
				klog.Errorf("Failed to patch node %v: %v", node.Name, err)
				errs = append(errs, err)
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%v", errs)
	}
	return nil
}

// deal with the cnrp curd
func (n *cnrpController) handles() cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    n.handleAdd,
		UpdateFunc: n.handleUpdate,
		DeleteFunc: n.handleDelete,
	}
}

func (n *cnrpController) handleAdd(obj interface{}) {
	n.enqueue(obj, cache.Added)
}

func (n *cnrpController) handleUpdate(old, new interface{}) {
	oldCNRP, newCNRP := old.(*schedulingapi.ClusterNodeResourcePolicy), new.(*schedulingapi.ClusterNodeResourcePolicy)

	if oldCNRP.ResourceVersion == newCNRP.ResourceVersion {
		return
	}

	n.enqueue(newCNRP, cache.Updated)
}

func (n *cnrpController) handleDelete(obj interface{}) {

	n.enqueue(obj, cache.Deleted)
}

func (n *cnrpController) enqueue(obj interface{}, action cache.DeltaType) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}

	klog.V(5).Infof("Enqueue CNRP %s, action %s", key, action)
	n.queue.Add(key)
}
