package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/component-base/version"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	schedulingapi "git.woa.com/crane/api/scheduling/v1alpha1"

	"github.com/gocrane/crane-scheduler/cmd/controller/app/config"
	"github.com/gocrane/crane-scheduler/cmd/controller/app/options"
	"github.com/gocrane/crane-scheduler/pkg/controller/annotator"
	"github.com/gocrane/crane-scheduler/pkg/metrics"
	_ "github.com/gocrane/crane-scheduler/pkg/plugins/apis/policy/scheme"
	pluginsapischeme "github.com/gocrane/crane-scheduler/pkg/plugins/apis/policy/scheme"
	"github.com/gocrane/crane-scheduler/pkg/webhooks"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	pluginsapischeme.AddToScheme(scheme)
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(schedulingapi.AddToScheme(scheme))
}

// NewControllerCommand creates a *cobra.Command object with default parameters
func NewControllerCommand(ctx context.Context) *cobra.Command {
	o, err := options.NewOptions()
	if err != nil {
		klog.Fatalf("unable to initialize command options: %v", err)
	}

	cmd := &cobra.Command{
		Use: "controller",
		Long: `The Crane Scheduler Controller is a kubernetes controller, which is used for annotating
		nodes with real load information sourced from metric datasource.`,
		Run: func(cmd *cobra.Command, args []string) {

			c, err := o.Config()
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}

			if err := Run(c.Complete(), ctx); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
	}

	err = o.Flags(cmd.Flags())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
	}

	return cmd
}

// Run executes controller based on the given configuration.
func Run(cc *config.CompletedConfig, ctx context.Context) error {

	klog.Infof("Starting Controller version %+v", version.Get())

	metrics.RegisterController()
	metrics.RegisterDataSource()

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		err := http.ListenAndServe(":8088", nil)
		if err != nil {
			klog.Error(err)
			return
		}
	}()

	// Setup a Manager
	if cc.WebhookConfig.Enabled {
		mgr, err := manager.New(cc.RestConfig, manager.Options{
			Scheme:             scheme,
			MetricsBindAddress: "0",
			Host:               cc.WebhookConfig.HookHost,
			Port:               cc.WebhookConfig.HookPort,
			CertDir:            cc.WebhookConfig.HookCertDir,
		})
		if err != nil {
			klog.Fatal(err)
		}

		// Setup webhooks
		hookServer := mgr.GetWebhookServer()

		if certDir := os.Getenv("WEBHOOK_CERT_DIR"); len(certDir) > 0 {
			hookServer.CertDir = certDir
		}

		hookServer.Register("/mutate-pod", &webhook.Admission{Handler: &webhooks.PodMutate{Client: mgr.GetClient()}})
		klog.Infof("webhook server started at %v:%v", hookServer.Host, hookServer.Port)
		go func() {
			if err := mgr.Start(ctx); err != nil {
				klog.Fatal(err)
			}
		}()
	}

	run := func(ctx context.Context) {
		annotatorController := annotator.NewNodeAnnotator(
			cc.KubeInformerFactory.Core().V1().Nodes(),
			cc.KubeInformerFactory.Core().V1().Events(),
			cc.CraneInformerFactory.Scheduling().V1alpha1().ClusterNodeResourcePolicies(),
			cc.CraneInformerFactory.Scheduling().V1alpha1().NodeResourcePolicies(),
			cc.KubeClient,
			cc.MetricsClient,
			*cc.Policy,
			cc.AnnotatorConfig.BindingHeapSize,
		)

		cc.KubeInformerFactory.Start(ctx.Done())
		cc.CraneInformerFactory.Start(ctx.Done())

		panic(annotatorController.Run(int(cc.AnnotatorConfig.ConcurrentSyncs), ctx.Done()))
	}

	if !cc.LeaderElection.LeaderElect {
		run(context.TODO())
		panic("unreachable")
	}

	id, err := os.Hostname()
	if err != nil {
		return err
	}

	// add a uniquifier so that two processes on the same host don't accidentally both become active
	id = id + "_" + string(uuid.NewUUID())
	rl, err := resourcelock.New(cc.LeaderElection.ResourceLock,
		cc.LeaderElection.ResourceNamespace,
		cc.LeaderElection.ResourceName,
		cc.LeaderElectionClient.CoreV1(),
		cc.LeaderElectionClient.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity:      id,
			EventRecorder: cc.EventRecorder,
		})
	if err != nil {
		panic(err)
	}

	electionChecker := leaderelection.NewLeaderHealthzAdaptor(time.Second * 20)
	leaderelection.RunOrDie(context.TODO(), leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: cc.LeaderElection.LeaseDuration.Duration,
		RenewDeadline: cc.LeaderElection.RenewDeadline.Duration,
		RetryPeriod:   cc.LeaderElection.RetryPeriod.Duration,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: run,
			OnStoppedLeading: func() {
				panic("leaderelection lost")
			},
		},
		WatchDog: electionChecker,
		Name:     "crane-scheduler-controller",
	})

	panic("unreachable")
}
