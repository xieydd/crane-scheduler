package options

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"gopkg.in/gcfg.v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	componentbaseconfig "k8s.io/component-base/config"
	"k8s.io/component-base/config/options"
	"k8s.io/klog/v2"

	craneclientset "git.woa.com/crane/api/pkg/generated/clientset/versioned"
	craneinfromers "git.woa.com/crane/api/pkg/generated/informers/externalversions"
	controllerappconfig "github.com/gocrane/crane-scheduler/cmd/controller/app/config"
	annotatorconfig "github.com/gocrane/crane-scheduler/pkg/controller/annotator/config"
	"github.com/gocrane/crane-scheduler/pkg/controller/metrics"
	"github.com/gocrane/crane-scheduler/pkg/datasource"
	"github.com/gocrane/crane-scheduler/pkg/datasource-providers/metricserver"
	"github.com/gocrane/crane-scheduler/pkg/datasource-providers/prom"
	"github.com/gocrane/crane-scheduler/pkg/datasource-providers/qcloudmonitor"
	dynamicscheduler "github.com/gocrane/crane-scheduler/pkg/plugins/dynamic"
	_ "github.com/gocrane/crane-scheduler/pkg/querybuilder-providers/metricserver"
	_ "github.com/gocrane/crane-scheduler/pkg/querybuilder-providers/prometheus"
	_ "github.com/gocrane/crane-scheduler/pkg/querybuilder-providers/qcloudmonitor"
	"github.com/gocrane/crane-scheduler/pkg/webhooks"
)

const (
	ControllerUserAgent = "crane-scheduler-controller"
)

// Options has all the params needed to run a Annotator.
type Options struct {
	*annotatorconfig.AnnotatorConfiguration

	LeaderElection *componentbaseconfig.LeaderElectionConfiguration

	master     string
	kubeconfig string

	WebhookConfig webhooks.Config
}

// NewOptions returns default annotator app options.
func NewOptions() (*Options, error) {
	o := &Options{
		AnnotatorConfiguration: &annotatorconfig.AnnotatorConfiguration{
			BindingHeapSize:  1024,
			ConcurrentSyncs:  4,
			PolicyConfigPath: "/etc/kubernetes/policy.yaml",
		},
		LeaderElection: &componentbaseconfig.LeaderElectionConfiguration{
			LeaderElect:       true,
			LeaseDuration:     metav1.Duration{Duration: 15 * time.Second},
			RenewDeadline:     metav1.Duration{Duration: 10 * time.Second},
			RetryPeriod:       metav1.Duration{Duration: 2 * time.Second},
			ResourceLock:      "leases",
			ResourceName:      "crane-scheduler-controller",
			ResourceNamespace: "crane-system",
		},
	}

	return o, nil
}

// Flags returns flags for a specific Annotator by section name.
func (o *Options) Flags(flag *pflag.FlagSet) error {
	if flag == nil {
		return fmt.Errorf("nil pointer")
	}

	flag.StringVar(&o.PolicyConfigPath, "policy-config-path", o.PolicyConfigPath, "Path to annotator policy config")
	flag.StringVar(&o.PrometheusAddr, "metrics-address", o.PrometheusAddr, "The address of metrics, from which we can pull metrics data.")
	flag.Int32Var(&o.BindingHeapSize, "binding-heap-size", o.BindingHeapSize, "Max size of binding heap size, used to store hot value data.")
	flag.Int32Var(&o.ConcurrentSyncs, "concurrent-syncs", o.ConcurrentSyncs, "The number of annotator controller workers that are allowed to sync concurrently.")
	flag.StringVar(&o.kubeconfig, "kubeconfig", o.kubeconfig, "Path to kubeconfig file with authorization information")
	flag.StringVar(&o.master, "master", o.master, "The address of the Kubernetes API server (overrides any value in kubeconfig)")
	flag.StringVar(&o.WebhookConfig.HookCertDir, "webhook-cert-dir", "", "webhook-cert-dir is the directory that contains the server key and certificate. "+
		"if not set, webhook server would look up the server key and certificate in "+
		"{TempDir}/k8s-webhook-server/serving-certs. The server key and certificate "+
		"must be named tls.key and tls.crt, respectively.")
	flag.StringVar(&o.WebhookConfig.HookHost, "webhook-host", "", "webhook host")
	flag.IntVar(&o.WebhookConfig.HookPort, "webhook-port", 9443, "webhook port")
	flag.BoolVar(&o.WebhookConfig.Enabled, "webhook-enabled", false, "webhook enabled")

	flag.StringVar(&o.DataSource, "datasource", "prom", "data source of the annotator, prom, qmonitor is available")

	flag.StringVar(&o.DataSourcePromConfig.Address, "prometheus-address", "", "prometheus address")
	flag.StringVar(&o.DataSourcePromConfig.Auth.Username, "prometheus-auth-username", "", "prometheus auth username")
	flag.StringVar(&o.DataSourcePromConfig.Auth.Password, "prometheus-auth-password", "", "prometheus auth password")
	flag.StringVar(&o.DataSourcePromConfig.Auth.BearerToken, "prometheus-auth-bearertoken", "", "prometheus auth bearertoken")
	flag.IntVar(&o.DataSourcePromConfig.QueryConcurrency, "prometheus-query-concurrency", 10, "prometheus query concurrency")
	flag.BoolVar(&o.DataSourcePromConfig.InsecureSkipVerify, "prometheus-insecure-skip-verify", false, "prometheus insecure skip verify")
	flag.DurationVar(&o.DataSourcePromConfig.KeepAlive, "prometheus-keepalive", 60*time.Second, "prometheus keep alive")
	flag.DurationVar(&o.DataSourcePromConfig.Timeout, "prometheus-timeout", 3*time.Minute, "prometheus timeout")
	flag.BoolVar(&o.DataSourcePromConfig.BRateLimit, "prometheus-bratelimit", false, "prometheus bratelimit")
	flag.IntVar(&o.DataSourcePromConfig.MaxPointsLimitPerTimeSeries, "prometheus-maxpoints", 11000, "prometheus max points limit per time series")
	flag.BoolVar(&o.DataSourcePromConfig.FederatedClusterScope, "prometheus-federated-cluster-scope", false, "prometheus support federated clusters query")
	flag.BoolVar(&o.DataSourcePromConfig.ThanosPartial, "prometheus-thanos-partial", false, "prometheus api to query thanos data source, hacking way, denote the thanos partial response query")
	flag.BoolVar(&o.DataSourcePromConfig.ThanosDedup, "prometheus-thanos-dedup", false, "prometheus api to query thanos data source, hacking way, denote the thanos deduplicate query")

	flag.StringVar(&o.DataSourceQMonitorConfig.ClusterId, "qmonitor-credentials-clusterid", "", "qcloud monitor clusterid which crane-scheduler installed on")
	flag.StringVar(&o.DataSourceQMonitorConfig.AppId, "qmonitor-credentials-appid", "", "qcloud monitor appid which crane-scheduler installed on")
	flag.StringVar(&o.DataSourceQMonitorConfig.Scheme, "qmonitor-clientprofile-scheme", "https", "qcloud monitor request scheme")
	flag.StringVar(&o.DataSourceQMonitorConfig.DomainSuffix, "qmonitor-clientprofile-domainsuffix", "internal.tencentcloudapi.com", "qcloud monitor client request domain suffix")
	flag.StringVar(&o.DataSourceQMonitorConfig.DefaultLanguage, "qmonitor-clientprofile-language", "zh-CN", "qcloud monitor client request default language")
	flag.IntVar(&o.DataSourceQMonitorConfig.DefaultTimeoutSeconds, "qmonitor-clientprofile-timeoutseconds", 15, "qcloud monitor client request default timeout seconds")
	flag.Int64Var(&o.DataSourceQMonitorConfig.DefaultLimit, "qmonitor-clientprofile-limit", 100, "qcloud monitor client request default language")
	flag.StringVar(&o.DataSourceQMonitorConfig.Region, "qmonitor-clientprofile-region", "", "qcloud monitor client request region")
	flag.BoolVar(&o.DataSourceQMonitorConfig.Debug, "qmonitor-clientprofile-debug", false, "qcloud monitor client request debug mode")

	flag.StringVar(&o.CloudConfig.Provider, "provider", "qcloud", "cloud provider the controller running on, now support qcloud.")
	flag.StringVar(&o.CloudConfig.CloudConfigFile, "cloudConfigFile", "", "cloudConfigFile specifies path for the cloud configuration.")

	options.BindLeaderElectionFlags(o.LeaderElection, flag)
	return nil
}

// ApplyTo fills up Annotator config with options.
func (o *Options) ApplyTo(c *controllerappconfig.Config) error {
	c.AnnotatorConfig = o.AnnotatorConfiguration
	c.LeaderElection = o.LeaderElection
	c.WebhookConfig = o.WebhookConfig
	return nil
}

// Validate validates the options and config before launching Annotator.
func (o *Options) Validate() error {
	switch strings.ToLower(o.DataSource) {
	case "metricserver", "ms":
		return nil
	case "qmonitor", "qcloudmonitor", "qm":
		if o.DataSourceQMonitorConfig.ClusterId == "" || o.DataSourceQMonitorConfig.AppId == "" {
			return fmt.Errorf("cloud config file `ClusterId` and `AppId` is required when use qmonitor in cloud by proxy user auth")
		}
	case "prometheus", "prom":
		return nil
	default:
		return fmt.Errorf("only support qmonitor, prometheus, %v not valid", o.DataSource)
	}

	return nil
}

func (o *Options) Complete() error {
	var cfg datasource.QCloudMonitorConfig
	if o.CloudConfig.CloudConfigFile != "" {
		cloudConfigFile, err := os.Open(o.CloudConfig.CloudConfigFile)
		defer cloudConfigFile.Close()
		if err != nil {
			return fmt.Errorf("couldn't open cloud provider configuration %s: %#v",
				o.CloudConfig.CloudConfigFile, err)
		}
		if err := gcfg.FatalOnly(gcfg.ReadInto(&cfg, cloudConfigFile)); err != nil {
			klog.Errorf("Failed to read TencentCloud configuration file: %v", err)
			return err
		}
		o.DataSourceQMonitorConfig = cfg
	}

	return nil
}

// Config returns an Annotator config object.
func (o *Options) Config() (*controllerappconfig.Config, error) {
	var kubeconfig *rest.Config
	var err error

	if err := o.Complete(); err != nil {
		return nil, err
	}

	if err := o.Validate(); err != nil {
		return nil, err
	}

	c := &controllerappconfig.Config{}
	if err := o.ApplyTo(c); err != nil {
		return nil, err
	}

	c.Policy, err = dynamicscheduler.LoadPolicyFromFile(o.PolicyConfigPath)
	if err != nil {
		return nil, err
	}

	if o.kubeconfig == "" {
		kubeconfig, err = rest.InClusterConfig()
	} else {
		// Build config from configfile
		kubeconfig, err = clientcmd.BuildConfigFromFlags(o.master, o.kubeconfig)
	}
	if err != nil {
		return nil, err
	}

	restConfig := rest.AddUserAgent(kubeconfig, ControllerUserAgent)

	c.RestConfig = restConfig

	c.KubeClient, err = clientset.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	c.CraneClient, err = craneclientset.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	realtime, history, _ := initializationDataSource(o, restConfig)

	c.MetricsClient = metrics.NewMetricClient(o.DataSourceQMonitorConfig.ClusterId, history, realtime)

	c.LeaderElectionClient = clientset.NewForConfigOrDie(rest.AddUserAgent(kubeconfig, "leader-election"))

	c.CraneInformerFactory = craneinfromers.NewSharedInformerFactory(c.CraneClient, 0)

	c.KubeInformerFactory = NewInformerFactory(c.KubeClient, 0)

	return c, nil
}

func initializationDataSource(opts *Options, restConfig *rest.Config) (datasource.RealTime, datasource.History, datasource.Interface) {
	var realtimeDataSource datasource.RealTime
	var historyDataSource datasource.History
	var hybridDataSource datasource.Interface
	datasourceStr := opts.DataSource
	switch strings.ToLower(datasourceStr) {
	case "metricserver", "ms":
		provider, err := metricserver.NewProvider(restConfig)
		if err != nil {
			klog.Exitf("unable to create datasource provider %v, err: %v", datasourceStr, err)
		}
		realtimeDataSource = provider
	case "qmonitor", "qcloudmonitor", "qm":
		provider, err := qcloudmonitor.NewProvider(&opts.DataSourceQMonitorConfig)
		if err != nil {
			klog.Exitf("unable to create datasource provider %v, err: %v", datasourceStr, err)
		}
		hybridDataSource = provider
		realtimeDataSource = provider
		historyDataSource = provider
	case "prometheus", "prom":
		fallthrough
	default:
		// default is prom
		provider, err := prom.NewProvider(&opts.DataSourcePromConfig)
		if err != nil {
			klog.Exitf("unable to create datasource provider %v, err: %v", datasourceStr, err)
		}
		hybridDataSource = provider
		realtimeDataSource = provider
		historyDataSource = provider
	}
	return realtimeDataSource, historyDataSource, hybridDataSource
}
