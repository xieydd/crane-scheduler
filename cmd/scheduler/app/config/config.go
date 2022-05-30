package config

import (
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"

	"github.com/gocrane/crane-scheduler/pkg/plugins/apis/policy"
	serverconfig "github.com/gocrane/crane-scheduler/pkg/server/config"
)

// Config is the main context object for crane scheduler controller.
type Config struct {
	// KubeInformerFactory gives access to kubernetes informers for the controller.
	KubeInformerFactory informers.SharedInformerFactory
	// KubeClient is the general kube client.
	KubeClient clientset.Interface
	// Policy is a collection of scheduler policies.
	Policy *policy.DynamicSchedulerPolicy

	// ServerConfig is the web server config for extender scheduler
	ServerCfg *serverconfig.Config
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
