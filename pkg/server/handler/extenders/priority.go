package extenders

import (
	"fmt"
	"k8s.io/klog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	schedulerextapi "k8s.io/kube-scheduler/extender/v1"

	"github.com/gocrane/crane-scheduler/pkg/known"
	"github.com/gocrane/crane-scheduler/pkg/metrics"
)

// priority extender

// Prioritize :
type Prioritize struct {
	Name string
	Func func(pod corev1.Pod, nodes []corev1.Node) (*schedulerextapi.HostPriorityList, error)
}

// Handler : because most of clusters are very small, less than 100 nodes, so we do not cache the nodes.
func (p Prioritize) Handler(c *gin.Context) {
	defer utilruntime.HandleCrash()
	var args schedulerextapi.ExtenderArgs
	var err error
	start := time.Now()
	defer func() {
		labels := map[string]string{
			"priority_name": known.PrioritySafeBalanceName,
		}
		metrics.ExtenderPriorityHandlerLatency.With(labels).Observe(time.Since(start).Seconds())
		metrics.RecordExtenderHandlerError(known.PrioritySafeBalanceName, args.Pod, err)
	}()

	err = c.BindJSON(&args)
	if err != nil {
		klog.Error(err, args)
		c.JSON(http.StatusOK, err)
		return
	}
	if args.Pod == nil {
		err = fmt.Errorf("no pod specified")
		klog.Error(err)
		c.JSON(http.StatusOK, err)
		return

	}
	if args.Nodes == nil {
		err = fmt.Errorf("do not support node cache")
		klog.Error(err)
		c.JSON(http.StatusOK, err)
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
