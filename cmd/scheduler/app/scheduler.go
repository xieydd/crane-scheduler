package app

import (
	"context"
	"flag"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"

	schedulingapi "git.woa.com/crane/api/scheduling/v1alpha1"

	"github.com/gocrane/crane-scheduler/cmd/scheduler/app/options"
	"github.com/gocrane/crane-scheduler/pkg/extenders"
	pluginsapischeme "github.com/gocrane/crane-scheduler/pkg/plugins/apis/policy/scheme"
	"github.com/gocrane/crane-scheduler/pkg/server"
)

var (
	scheme = runtime.NewScheme()
)

// NewCraneSchedulerCommand creates a *cobra.Command object with default parameters
func NewCraneSchedulerCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use:  "craned",
		Long: `The crane scheduler is used to do scheduling, it supports extender scheduler mode or plugin-complied mode`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := opts.Complete(); err != nil {
				klog.Exit(err)
			}
			if errs := opts.Validate(); len(errs) != 0 {
				klog.Exit(errs)
			}

			if err := Run(ctx, opts); err != nil {
				klog.Exit(err)
			}
		},
	}

	cmd.Flags().AddGoFlagSet(flag.CommandLine)
	opts.AddFlags(cmd.Flags())
	utilfeature.DefaultMutableFeatureGate.AddFlag(cmd.Flags())

	return cmd
}

// Run runs the crane scheduler with options. This should never exit.
func Run(ctx context.Context, opts *options.Options) error {
	initializationScheme()
	cfg, err := opts.Config()
	if err != nil {
		return err
	}
	extScheduler := extenders.NewExtenderScheduler(cfg.Policy.Spec, cfg.KubeInformerFactory.Core().V1().ConfigMaps())
	cfg.ServerCfg.ExtenderScheduler = extScheduler
	// use controller runtime rest config, we can not refer kubeconfig option directly because it is unexported variable in vendor/sigs.k8s.io/controller-runtime/pkg/client/config/config.go
	craneServer, err := server.NewServer(cfg.ServerCfg)
	if err != nil {
		klog.Exit(err)
	}

	var eg errgroup.Group

	eg.Go(func() error {
		cfg.KubeInformerFactory.Start(ctx.Done())
		return extScheduler.Run(ctx.Done())
	})

	eg.Go(func() error {
		// Start the craned web server
		craneServer.Run(ctx)
		return nil
	})

	// wait for all components exit
	if err := eg.Wait(); err != nil {
		klog.Fatal(err)
	}

	return nil
}

func initializationScheme() {
	pluginsapischeme.AddToScheme(scheme)
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(schedulingapi.AddToScheme(scheme))
}
