package extenders

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
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
	defer func() {
		labels := map[string]string{
			"predicate_name": known.PredicateSafeOverloadName,
		}
		metrics.ExtenderPredicateHandlerLatency.With(labels).Observe(time.Since(start).Seconds())
	}()
	var args schedulerextapi.ExtenderArgs
	err := c.BindJSON(args)

	if err != nil {
		extenderFilterResult := &schedulerextapi.ExtenderFilterResult{
			Nodes:       nil,
			FailedNodes: nil,
			Error:       err.Error(),
		}
		c.JSON(http.StatusOK, extenderFilterResult)
		return
	}
	if args.Pod == nil {
		extenderFilterResult := &schedulerextapi.ExtenderFilterResult{
			Nodes:       nil,
			FailedNodes: nil,
			Error:       fmt.Errorf("no pod specified").Error(),
		}
		c.JSON(http.StatusOK, extenderFilterResult)
		return
	}
	if args.Nodes == nil {
		extenderFilterResult := &schedulerextapi.ExtenderFilterResult{
			Nodes:       nil,
			FailedNodes: nil,
			Error:       fmt.Errorf("do not support node cache").Error(),
		}
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
		c.JSON(http.StatusOK, extenderFilterResult)
		return
	}

	c.JSON(http.StatusOK, extenderFilterResult)
}
