package main

import (
	"context"
	goflag "flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	v1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	schedulingapi "git.woa.com/crane/api/scheduling/v1alpha1"

	"github.com/gocrane/crane-scheduler/pkg/utils"
)

var (
	scheme = runtime.NewScheme()
)

type Options struct {
	Master                              string
	Kubeconfig                          string
	ClearNodeAnnotationPrefix           []string
	DeploymentControllerToStopNamespace []string
	DeploymentControllerToStop          []string
}

// Flags returns flags for a specific Annotator by section name.
func (o *Options) Flags(flag *pflag.FlagSet) error {
	if flag == nil {
		return fmt.Errorf("nil pointer")
	}

	flag.StringVar(&o.Master, "master", "", "kubernetes master endpoint")
	flag.StringVar(&o.Kubeconfig, "kubeconfig", "", "kubernetes kubeconfig path")
	flag.StringSliceVar(&o.DeploymentControllerToStopNamespace, "deployment-controller-to-stop-namespace", []string{}, "deployment controller need to be stop by scaling to zero to avoid it patch node again after helmhook clear it, this hook is execute when helm uninstall, so it is ok")
	flag.StringSliceVar(&o.DeploymentControllerToStop, "deployment-controller-to-stop", []string{}, "deployment controller need to be stop by scaling to zero to avoid it patch node again after helmhook clear it, this hook is execute when helm uninstall, so it is ok")
	flag.StringSliceVar(&o.ClearNodeAnnotationPrefix, "clear-node-annotation-prefix", []string{}, "denote which annotations contains the specified prefix to be cleared")

	return nil
}

func NewCommand(ctx context.Context) *cobra.Command {
	opts := Options{}

	cmd := &cobra.Command{
		Use:   "helmhook",
		Short: "helmhook for clear crane patched node annotations after uninstalled the crane",
		Run: func(cmd *cobra.Command, args []string) {

			if err := Run(ctx, opts); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
	}
	err := opts.Flags(cmd.Flags())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
	}

	return cmd
}

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(schedulingapi.AddToScheme(scheme))
}

// craned main.
func main() {
	rand.Seed(time.Now().UTC().UnixNano())

	klog.InitFlags(nil)

	pflag.CommandLine.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	logs.InitLogs()
	defer logs.FlushLogs()

	ctx := signals.SetupSignalHandler()

	if err := NewCommand(ctx).Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		klog.Fatal(err)
	}
}

func Run(ctx context.Context, opt Options) error {
	var kubeconfig *rest.Config
	var err error

	if opt.Kubeconfig == "" {
		kubeconfig, err = rest.InClusterConfig()
	} else {
		// Build config from configfile
		kubeconfig, err = clientcmd.BuildConfigFromFlags(opt.Master, opt.Kubeconfig)
	}
	if err != nil {
		return err
	}

	restConfig := rest.AddUserAgent(kubeconfig, "crane-helm-hook")

	kubeClient, err := clientset.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	//craneClient, err := craneclientset.NewForConfig(restConfig)
	//if err != nil {
	//	return err
	//}

	// 1. first stop the crane-scheduler-controller, scale it to zero replica to avoid it patch node again
	namespaces := opt.DeploymentControllerToStopNamespace
	controllers := opt.DeploymentControllerToStop
	for i := range namespaces {
		if i < len(controllers) {
			_, err = kubeClient.AppsV1().Deployments(namespaces[i]).UpdateScale(ctx, controllers[i], &v1.Scale{
				Spec: v1.ScaleSpec{
					Replicas: 0,
				}}, metav1.UpdateOptions{})
			if err != nil {
				klog.Errorf("Failed to update scale for deployment %v/%v, err: %v", namespaces[i], controllers[i], err)
				continue
			}
			err = waitForDeploymentScaleToZero(ctx, kubeClient, namespaces[i], controllers[i], 3*time.Minute)
			if err != nil {
				klog.Errorf("Failed to update waitForDeploymentScaleToZero for deployment %v/%v, err: %v", namespaces[i], controllers[i], err)
				continue
			}
		}
	}

	// 2. clear all node crane annotations

	nodeMap := make(map[string]corev1.Node)
	allNodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, node := range allNodes.Items {
		nodeMap[node.Name] = node
	}

	dswAnns := make(map[string]string)
	for name, node := range nodeMap {
		aswAnns := utils.GetNodeAnnotationWithPrefix(&node, opt.ClearNodeAnnotationPrefix)
		if !equality.Semantic.DeepEqual(&aswAnns, &dswAnns) {
			patchData, err := utils.BuildPatchBytes(aswAnns, dswAnns)
			if err != nil {
				klog.Errorf("Failed to BuildPatchBytes for node %v, aswAnns: %v, dswAnns: %v, err: %v", name, aswAnns, dswAnns, err)
				continue
			}
			_, err = kubeClient.CoreV1().Nodes().Patch(ctx, node.Name, types.JSONPatchType, patchData, metav1.PatchOptions{})
			if err != nil {
				klog.Errorf("Failed to patch node %v: %v", node.Name, err)
				continue
			}
			klog.Infof("Succeed to clear node %v annotations: %v", node.Name, aswAnns)
		}
	}

	//cnrpList, err := craneClient.SchedulingV1alpha1().ClusterNodeResourcePolicies().List(context.TODO(), metav1.ListOptions{})
	//if err == nil {
	//	for _, cnrp := range cnrpList.Items {
	//		selector, err := metav1.LabelSelectorAsSelector(&cnrp.Spec.NodeSelector)
	//		if err != nil {
	//			klog.Error(err)
	//			continue
	//		}
	//		nodes, err :=  kubeClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
	//		if err != nil {
	//			klog.Error(err)
	//			continue
	//		}
	//		for _, node := range nodes.Items {
	//			nodeMap[node.Name] = node
	//		}
	//	}
	//} else {
	//	allNodes, err := kubeClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	//	if err != nil {
	//		klog.Fatal(err)
	//	}
	//	for _, node := range allNodes.Items {
	//		nodeMap[node.Name] = node
	//	}
	//}
	//
	return nil
}

func waitForDeploymentScaleToZero(ctx context.Context, kubeClient clientset.Interface, namespace, name string, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	n := 1
	max := 30
	for {
		select {
		case <-timer.C:
			return fmt.Errorf("timeout")
		default:
			scale, err := kubeClient.AppsV1().Deployments(namespace).GetScale(ctx, name, metav1.GetOptions{})
			if err != nil {
				klog.Errorf("Failed to get scale for deployment %v/%v, err: %v", namespace, name, err)
				continue
			}
			podsList, err := kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: scale.Status.Selector})
			if err != nil {
				klog.Errorf("Failed to get scale for deployment %v/%v, err: %v", namespace, name, err)
				continue
			}
			if len(podsList.Items) == 0 {
				return nil
			} else {
				if n > max {
					n = max
				}
				time.Sleep(time.Duration(n) * time.Second)
				n = n * 2
			}
		}
	}
}
