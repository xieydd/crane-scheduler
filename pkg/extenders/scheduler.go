package extenders

import (
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1informer "k8s.io/client-go/informers/core/v1"
	corev1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	schedulerextapi "k8s.io/kube-scheduler/extender/v1"

	"github.com/gocrane/crane-scheduler/pkg/extenders/safebalance"
	"github.com/gocrane/crane-scheduler/pkg/extenders/safeoverload"
	"github.com/gocrane/crane-scheduler/pkg/known"
	"github.com/gocrane/crane-scheduler/pkg/plugins/apis/policy"
)

type PredicateFunc func(pod corev1.Pod, nodes []corev1.Node) (*schedulerextapi.ExtenderFilterResult, error)
type PriorityFunc func(pod corev1.Pod, nodes []corev1.Node) (*schedulerextapi.HostPriorityList, error)

type ExtenderSchedulerPredicateFunc func(pod corev1.Pod, nodes []corev1.Node, policySpec policy.PolicySpec) (*schedulerextapi.ExtenderFilterResult, error)
type ExtenderSchedulerPriorityFunc func(pod corev1.Pod, nodes []corev1.Node, policySpec policy.PolicySpec) (*schedulerextapi.HostPriorityList, error)

type ExtenderScheduler struct {
	// todo: later we use a webhook to watch global config, then patch annotation to pod to decide weather to apply crane scheduler,
	//   this way the scheduler can not care about how to sync the user apply scope config
	cmInformer       corev1informer.ConfigMapInformer
	cmInformerSynced cache.InformerSynced
	cmLister         corev1lister.ConfigMapLister

	lock                 sync.Mutex
	namespacesApplyScope map[string]bool
	PolicySpec           policy.PolicySpec
	Predicates           map[string]ExtenderSchedulerPredicateFunc
	Priorities           map[string]ExtenderSchedulerPriorityFunc
}

func NewExtenderScheduler(policySpec policy.PolicySpec, cmInformer corev1informer.ConfigMapInformer) *ExtenderScheduler {
	return &ExtenderScheduler{
		cmInformer:       cmInformer,
		cmInformerSynced: cmInformer.Informer().HasSynced,
		cmLister:         cmInformer.Lister(),
		PolicySpec:       policySpec,
		Predicates: map[string]ExtenderSchedulerPredicateFunc{
			known.PredicateSafeOverloadName: safeoverload.PredictateFunc,
		},
		Priorities: map[string]ExtenderSchedulerPriorityFunc{
			known.PrioritySafeBalanceName: safebalance.PriorityFunc,
		},
	}
}

func (es *ExtenderScheduler) Run(stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()

	if !cache.WaitForCacheSync(stopCh, es.cmInformerSynced) {
		return fmt.Errorf("failed to wait for cache sync for scheduler")
	}
	klog.Info("Caches are synced for scheduler")

	cmController := newConfigmapController(es)
	es.cmInformer.Informer().AddEventHandler(cmController.handles())
	for i := 0; i < 2; i++ {
		go wait.Until(cmController.Run, time.Second, stopCh)
	}

	<-stopCh

	return nil
}

func (es *ExtenderScheduler) UpdateNamespaceApplyScope(namespaces map[string]bool) {
	es.lock.Lock()
	defer es.lock.Unlock()
	es.namespacesApplyScope = namespaces
}

func (es *ExtenderScheduler) GetPredicatesFunc(name string) PredicateFunc {
	return func(pod corev1.Pod, nodes []corev1.Node) (*schedulerextapi.ExtenderFilterResult, error) {
		es.lock.Lock()
		defer es.lock.Unlock()

		f, ok := es.Predicates[name]
		if ok {
			return f(pod, nodes, es.PolicySpec)
		}
		return nil, fmt.Errorf("not supported extender predicate func: %v", name)
	}
}

func (es *ExtenderScheduler) GetPrioritiesFunc(name string) PriorityFunc {
	return func(pod corev1.Pod, nodes []corev1.Node) (*schedulerextapi.HostPriorityList, error) {
		es.lock.Lock()
		defer es.lock.Unlock()

		f, ok := es.Priorities[name]
		if ok {
			return f(pod, nodes, es.PolicySpec)
		}
		return nil, fmt.Errorf("not supported extender priority func: %v", name)
	}
}
