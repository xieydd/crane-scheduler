package extenders

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"github.com/gocrane/crane-scheduler/pkg/known"
	"github.com/gocrane/crane-scheduler/pkg/utils"
)

type cmController struct {
	es    *ExtenderScheduler
	queue workqueue.RateLimitingInterface
}

func newConfigmapController(es *ExtenderScheduler) *cmController {
	cmRateLimiter := workqueue.NewItemExponentialFailureRateLimiter(known.DefaultBackOff,
		known.MaxBackOff)

	return &cmController{
		es:    es,
		queue: workqueue.NewNamedRateLimitingQueue(cmRateLimiter, "configmap_queue"),
	}
}

func (n *cmController) Run() {
	defer n.queue.ShutDown()
	defer runtime.HandleCrash()
	klog.Infof("Start to reconcile configmap")

	for n.processNextWorkItem() {
	}
}

func (n *cmController) processNextWorkItem() bool {
	key, quit := n.queue.Get()
	if quit {
		return false
	}
	defer n.queue.Done(key)

	forget, err := n.syncConfigMap(key.(string))
	if err != nil {
		klog.Warningf("Failed to sync this configmap [%q]: %v", key.(string), err)
	}
	if forget {
		n.queue.Forget(key)
		return true
	}

	n.queue.AddRateLimited(key)

	return true
}

func (n *cmController) syncConfigMap(key string) (bool, error) {
	startTime := time.Now()
	defer func() {
		klog.Infof("Finished syncing configmap[%s] (%v)", key, time.Since(startTime))
	}()

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return true, fmt.Errorf("invalid resource key: %s", key)
	}

	config, err := n.es.cmLister.ConfigMaps(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			n.es.UpdateNamespaceApplyScope(map[string]bool{})
			return true, fmt.Errorf("can not find configmap[%s]: %v, maybe already deleted", key, err)
		} else {
			return false, fmt.Errorf("failed to get configmap[%s]: %v", key, err)
		}
	}

	namespaces := utils.GetSchedulerNamespaceApplyScope(config)

	klog.Infof("Sync configmap[%s], got namespaceScope: %+v", key, namespaces)

	n.es.UpdateNamespaceApplyScope(namespaces)

	return true, nil
}

func (n *cmController) handles() cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    n.handleAdd,
		UpdateFunc: n.handleUpdate,
		DeleteFunc: n.handleDelete,
	}
}

func (n *cmController) handleAdd(obj interface{}) {
	n.enqueue(obj, cache.Added)
}

func (n *cmController) handleUpdate(old, new interface{}) {
	oldCNRP, newCNRP := old.(*corev1.ConfigMap), new.(*corev1.ConfigMap)

	if oldCNRP.ResourceVersion == newCNRP.ResourceVersion {
		return
	}

	n.enqueue(newCNRP, cache.Updated)
}

func (n *cmController) handleDelete(obj interface{}) {

	n.enqueue(obj, cache.Deleted)
}

func (n *cmController) enqueue(obj interface{}, action cache.DeltaType) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}

	klog.V(5).Infof("Enqueue configmap %s, action %s", key, action)
	n.queue.Add(key)
}
