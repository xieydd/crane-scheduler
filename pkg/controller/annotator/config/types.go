package config

import "github.com/gocrane/crane-scheduler/pkg/datasource"

// AnnotatorConfiguration holds configuration for a node annotator.
type AnnotatorConfiguration struct {
	// BindingHeapSize limits the size of Binding Heap, which stores the lastest
	// pod scheduled information.
	BindingHeapSize int32
	// ConcurrentSyncs specified the number of annotator controller workers.
	ConcurrentSyncs int32
	// PolicyConfigPath specified the path of Scheduler Policy File.
	PolicyConfigPath string
	// PrometheusAddr is the address of Prometheus Service.
	PrometheusAddr string

	// CloudConfig is the cloud provider config
	CloudConfig CloudConfig

	// DataSource is the datasource type, prometheus or qcloud monitor
	DataSource string
	// DataSourcePromConfig is the prometheus datasource config
	DataSourcePromConfig datasource.PromConfig
	// DataSourceQMonitorConfig is the tencent cloud monitor datasource config, actually it is cloud provider secrets
	DataSourceQMonitorConfig datasource.QCloudMonitorConfig
}

type CloudConfig struct {
	CloudConfigFile string `json:"cloudConfigFile"`
	Provider        string `json:"provider"`
}
