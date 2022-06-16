package config

import (
	"github.com/gocrane/crane-scheduler/pkg/webhooks"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"

	componentbaseconfig "k8s.io/component-base/config"

	policy "github.com/gocrane/crane-scheduler/pkg/plugins/apis/policy"

	craneclientset "git.woa.com/crane/api/pkg/generated/clientset/versioned"
	craneinfromers "git.woa.com/crane/api/pkg/generated/informers/externalversions"
	annotatorconfig "github.com/gocrane/crane-scheduler/pkg/controller/annotator/config"
	"github.com/gocrane/crane-scheduler/pkg/controller/metrics"
)

// Config is the main context object for crane scheduler controller.
type Config struct {
	// AnnotatorConfig holds configuration for a node annotator.
	AnnotatorConfig *annotatorconfig.AnnotatorConfiguration
	// LeaderElection holds configuration for leader election.
	LeaderElection *componentbaseconfig.LeaderElectionConfiguration
	// KubeInformerFactory gives access to kubernetes informers for the controller.
	KubeInformerFactory informers.SharedInformerFactory
	// CraneInformerFactory gives access to crane scheduling informers for the controller.
	CraneInformerFactory craneinfromers.SharedInformerFactory
	// CraneClient is the crane crd client.
	CraneClient craneclientset.Interface
	// KubeClient is the general kube client.
	KubeClient clientset.Interface
	// RestConfig
	RestConfig *rest.Config
	// MetricsClient is used for getting metric data from data source.
	MetricsClient metrics.MetricClient
	// Policy is a collection of scheduler policies.
	Policy *policy.DynamicSchedulerPolicy
	// EventRecorder is the event sink
	EventRecorder record.EventRecorder
	// LeaderElectionClient is the client used for leader election
	LeaderElectionClient *clientset.Clientset
	// WebhookConfig
	WebhookConfig webhooks.Config
}

type completedConfig struct {
	*Config
}

// CompletedConfig same as Config, just to swap private object.
type CompletedConfig struct {
	// Embed a private pointer that cannot be instantiated outside of this package.
	*completedConfig
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (c *Config) Complete() *CompletedConfig {
	cc := completedConfig{c}

	return &CompletedConfig{&cc}
}
