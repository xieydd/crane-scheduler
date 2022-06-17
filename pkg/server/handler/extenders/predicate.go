package extenders

import (
	"fmt"
	"k8s.io/klog/v2"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	schedulerextapi "k8s.io/kube-scheduler/extender/v1"

	"github.com/gocrane/crane-scheduler/pkg/known"
	"github.com/gocrane/crane-scheduler/pkg/metrics"
)

// predicate extender

// Predicate :
type Predicate struct {
	Name string
	Func func(pod corev1.Pod, nodes []corev1.Node) (*schedulerextapi.ExtenderFilterResult, error)
}

// Handler : because most of clusters are very small, less than 100 nodes, so we do not cache the nodes.
func (p Predicate) Handler(c *gin.Context) {
	defer utilruntime.HandleCrash()
	start := time.Now()
	var err error
	var args schedulerextapi.ExtenderArgs
	defer func() {

		metrics.ExtenderPredicateHandlerLatency.With(
			prometheus.Labels{"predicate_name": known.PredicateSafeOverloadName},
		).Observe(time.Since(start).Seconds())

		metrics.RecordExtenderHandlerError(known.PredicateSafeOverloadName, args.Pod, err)
	}()
	err = c.BindJSON(&args)
	if err != nil {
		klog.Error(err, args)
		extenderFilterResult := &schedulerextapi.ExtenderFilterResult{
			Nodes:       nil,
			FailedNodes: nil,
			Error:       err.Error(),
		}
		c.JSON(http.StatusOK, extenderFilterResult)
		return
	}
	if args.Pod == nil {
		err = fmt.Errorf("no pod specified")
		extenderFilterResult := &schedulerextapi.ExtenderFilterResult{
			Nodes:       nil,
			FailedNodes: nil,
			Error:       err.Error(),
		}
		klog.Error(err, args)
		c.JSON(http.StatusOK, extenderFilterResult)
		return
	}
	if args.Nodes == nil {
		err = fmt.Errorf("do not support node cache")
		extenderFilterResult := &schedulerextapi.ExtenderFilterResult{
			Nodes:       nil,
			FailedNodes: nil,
			Error:       err.Error(),
		}
		klog.Error(err, args)
		c.JSON(http.StatusOK, extenderFilterResult)
		return
	}
	extenderFilterResult, err := p.Func(*args.Pod, args.Nodes.Items)
	if err != nil {
		extenderFilterResult := &schedulerextapi.ExtenderFilterResult{
			Nodes:       nil,
			FailedNodes: nil,
			Error:       err.Error(),
		}
		klog.Error(err, args)
		c.JSON(http.StatusOK, extenderFilterResult)
		return
	}

	c.JSON(http.StatusOK, extenderFilterResult)
}
