package config

import (
	"k8s.io/client-go/rest"

	"github.com/gocrane/crane-scheduler/pkg/extenders"
)

type Config struct {
	Mode        string `json:"mode"`
	BindAddress string `json:"bindAddress"`
	BindPort    int    `json:"bindPort"`

	EnableProfiling bool `json:"profiling"`
	EnableMetrics   bool `json:"enableMetrics"`

	KubeRestConfig *rest.Config `json:"KubeRestConfig"`

	ExtenderScheduler *extenders.ExtenderScheduler
}

func NewServerConfig() *Config {
	return &Config{}
}
