package options

import (
	"time"

	"k8s.io/apimachinery/pkg/fields"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/gocrane/crane-scheduler/pkg/known"
)

// NewInformerFactory creates a SharedInformerFactory and initializes an configmap informer that returns specified events.
func NewInformerFactory(cs clientset.Interface, resyncPeriod time.Duration) informers.SharedInformerFactory {
	informerFactory := informers.NewSharedInformerFactory(cs, resyncPeriod)

	informerFactory.InformerFor(&v1.ConfigMap{}, newConfigMapInformer)

	return informerFactory
}

// newEventInformer creates a shared index informer that returns only scheduled and normal event.
func newConfigMapInformer(cs clientset.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	tweakListOptions := func(options *metav1.ListOptions) {
		options.FieldSelector = fields.OneTermEqualSelector("metadata.name", known.ConfigMapSchedulerApplyScope).String()
	}

	return coreinformers.NewFilteredConfigMapInformer(cs, metav1.NamespaceSystem, resyncPeriod, nil, tweakListOptions)
}
