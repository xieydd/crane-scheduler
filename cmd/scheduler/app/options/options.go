package options

import (
	"flag"
	"fmt"
	"time"

	"github.com/spf13/pflag"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	componentbaseconfig "k8s.io/component-base/config"
	scheduleropts "k8s.io/kubernetes/cmd/kube-scheduler/app/options"

	schedulerappconfig "github.com/gocrane/crane-scheduler/cmd/scheduler/app/config"
	dynamicscheduler "github.com/gocrane/crane-scheduler/pkg/plugins/dynamic"
	serverconfig "github.com/gocrane/crane-scheduler/pkg/server/config"
)

const (
	SchedulerUserAgent = "crane-scheduler"
)

// Options hold the command-line options about craned
type Options struct {
	// ApiQps for rest client
	ApiQps int
	// ApiBurst for rest  client
	ApiBurst int
	// LeaderElection hold the configurations for manager leader election.
	LeaderElection componentbaseconfig.LeaderElectionConfiguration

	// PolicyConfigPath specified the path of Scheduler Policy File.
	PolicyConfigPath string

	Kubeconfig string
	Master     string

	// ServerOptions hold the web server options
	ServerOptions *ServerOptions

	scheduleropts *scheduleropts.Options
}

// NewOptions builds an empty options.
func NewOptions() *Options {
	return &Options{
		ServerOptions: NewServerOptions(),
		scheduleropts: scheduleropts.NewOptions(),
	}
}

// Complete completes all the required options.
func (o *Options) Complete() error {
	return o.ServerOptions.Complete()
}

// Validate all required options.
func (o *Options) Validate() []error {
	return o.ServerOptions.Validate()
}

func (o *Options) ApplyTo(cfg *schedulerappconfig.Config) error {
	return o.ServerOptions.ApplyTo(cfg.ServerCfg)
}

// Config returns an Annotator config object.
func (o *Options) Config() (*schedulerappconfig.Config, error) {
	var kubeconfig *rest.Config
	var err error

	if err := o.Complete(); err != nil {
		return nil, err
	}

	if errs := o.Validate(); len(errs) > 0 {
		return nil, fmt.Errorf("%v", errs)
	}

	c := &schedulerappconfig.Config{
		ServerCfg: serverconfig.NewServerConfig(),
	}
	if err := o.ApplyTo(c); err != nil {
		return nil, err
	}

	c.Policy, err = dynamicscheduler.LoadPolicyFromFile(o.PolicyConfigPath)
	if err != nil {
		return nil, err
	}

	if o.Kubeconfig == "" {
		kubeconfig, err = rest.InClusterConfig()
	} else {
		// Build config from config file
		kubeconfig, err = clientcmd.BuildConfigFromFlags(o.Master, o.Kubeconfig)
	}
	if err != nil {
		return nil, err
	}

	restConfig := rest.AddUserAgent(kubeconfig, SchedulerUserAgent)

	c.ServerCfg.KubeRestConfig = restConfig

	c.KubeClient, err = clientset.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	c.KubeInformerFactory = NewInformerFactory(c.KubeClient, 0)

	return c, nil
}

// AddFlags adds flags to the specified FlagSet.
func (o *Options) AddFlags(flags *pflag.FlagSet) {
	o.ServerOptions.AddFlags(flags)

	flags.IntVar(&o.ApiQps, "api-qps", 300, "QPS of rest config.")
	flags.IntVar(&o.ApiBurst, "api-burst", 400, "Burst of rest config.")
	flags.BoolVar(&o.LeaderElection.LeaderElect, "leader-elect", true, "Start a leader election client and gain leadership before executing the main loop. Enable this when running replicated components for high availability.")
	flags.DurationVar(&o.LeaderElection.LeaseDuration.Duration, "lease-duration", 15*time.Second,
		"Specifies the expiration period of lease.")
	flags.DurationVar(&o.LeaderElection.RetryPeriod.Duration, "lease-retry-period", 2*time.Second,
		"Specifies the lease renew interval.")
	flags.DurationVar(&o.LeaderElection.RenewDeadline.Duration, "lease-renew-period", 10*time.Second,
		"Specifies the lease renew interval.")

	flags.StringVar(&o.PolicyConfigPath, "policy-config-path", o.PolicyConfigPath, "Path to scheduler policy config")
	flag.StringVar(&o.Kubeconfig, "kubeconfig", o.Kubeconfig, "Path to kubeconfig file with authorization information")
	flag.StringVar(&o.Master, "master", o.Master, "The address of the Kubernetes API server (overrides any value in kubeconfig)")
}
