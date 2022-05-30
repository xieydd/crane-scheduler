package qcloudmonitor

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/regions"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/util/flowcontrol"

	"github.com/gocrane/crane-scheduler/pkg/cloudsdk/qcloud"
	qconsts "github.com/gocrane/crane-scheduler/pkg/cloudsdk/qcloud/consts"
	"github.com/gocrane/crane-scheduler/pkg/cloudsdk/qcloud/credential"
	"github.com/gocrane/crane-scheduler/pkg/cloudsdk/qcloud/qmonitor"
	"github.com/gocrane/crane-scheduler/pkg/datasource"
	"github.com/gocrane/crane-scheduler/pkg/known"
	"github.com/gocrane/crane-scheduler/pkg/metricnaming"
	_ "github.com/gocrane/crane-scheduler/pkg/querybuilder-providers/metricserver"
	_ "github.com/gocrane/crane-scheduler/pkg/querybuilder-providers/prometheus"
	_ "github.com/gocrane/crane-scheduler/pkg/querybuilder-providers/qcloudmonitor"
)

func GetAssumedCredentialFromEnv() (id, key, token string) {
	id = os.Getenv("AssumedKeyId")
	key = os.Getenv("AssumedKeySecret")
	token = os.Getenv("AssumedToken")
	return
}

func TestQueryTimeSeries(t *testing.T) {
	clusterid := "cls-rfm3e85h"

	config := &datasource.QCloudMonitorConfig{
		Credentials: datasource.Credentials{},
		ClientProfile: datasource.ClientProfile{
			Region: regions.Toronto,
			Debug:  true,
		},
	}
	FillDefaultValue(config)
	cred := credential.NewQCloudNormCredential(config.ClusterId, config.AppId, config.SecretId, config.SecretKey, 1*time.Hour)
	cred.DisableRefresh()
	qcp := qcloud.QCloudClientProfile{
		Region:          config.Region,
		DomainSuffix:    config.DomainSuffix,
		Scheme:          config.Scheme,
		DefaultLimit:    config.DefaultLimit,
		DefaultLanguage: config.DefaultLanguage,
		DefaultTimeout:  time.Duration(config.DefaultTimeoutSeconds) * time.Second,
		Debug:           config.Debug,
	}
	qclouClientConf := &qcloud.QCloudClientConfig{
		DefaultRetryCnt:     qconsts.MAXRETRY,
		Credential:          cred,
		QCloudClientProfile: qcp,
		RateLimiter:         flowcontrol.NewTokenBucketRateLimiter(10, 20),
	}
	cmClient := qmonitor.NewQCloudMonitorClient(qclouClientConf)
	provider, err := NewProviderByCmClient(cmClient)
	if err != nil {
		t.Fatal(err)
	}
	cmClient.UpdateQCloudAssumedCredential(GetAssumedCredentialFromEnv())

	nodeName := "10.0.2.250"
	nodeIp := "10.0.2.250"
	testCases := []struct {
		desc       string
		metricname string
		nodeName   string
		nodeIp     string
	}{
		{
			desc:       "tc1",
			metricname: known.MetricCpuUsageMaxAvg1hPercent,
			nodeName:   nodeName,
			nodeIp:     nodeIp,
		},

		{
			desc:       "tc2",
			metricname: known.MetricCpuUsageMaxAvg1dPercent,
			nodeName:   nodeName,
			nodeIp:     nodeIp,
		},
		{
			desc:       "tc3",
			metricname: known.MetricCpuUsagePercent,
			nodeName:   nodeName,
			nodeIp:     nodeIp,
		},

		{
			desc:       "tc4",
			metricname: known.MetricCpuUsageAvg5mPercent,
			nodeName:   nodeName,
			nodeIp:     nodeIp,
		},

		//{
		//	desc:       "tc5",
		//	metricname: known.MetricMemUsageMaxAvg1hPercent,
		//	nodeName:   nodeName,
		//	nodeIp:     nodeIp,
		//},
		//{
		//	desc:       "tc6",
		//	metricname: known.MetricMemUsageMaxAvg1dPercent,
		//	nodeName:   nodeName,
		//	nodeIp:     nodeIp,
		//},
		//{
		//	desc:       "tc7",
		//	metricname: known.MetricMemUsagePercent,
		//	nodeName:   nodeName,
		//	nodeIp:     nodeIp,
		//},
		//{
		//	desc:       "tc8",
		//	metricname: known.MetricMemUsageAvg5mPercent,
		//	nodeName:   nodeName,
		//	nodeIp:     nodeIp,
		//},
	}
	for _, tc := range testCases {
		namer := metricnaming.NodeMetricNamer(clusterid, nodeName, nodeIp, "", "Node", tc.metricname, labels.Everything())
		end := time.Date(2022, 6, 10, 13, 45, 0, 0, time.Local)
		step := 60 * time.Second
		start := end.Add(-30 * step)
		results, err := provider.QueryTimeSeries(context.TODO(), namer, start, end, step)
		if err != nil {
			t.Errorf("tc %v, err: %v", tc.desc, err)
			continue
		}
		m := len(results)
		fmt.Println(m)
		if m == 0 {
			t.Errorf("tc %v, err: %v", tc.desc, fmt.Errorf("history query time series is null, clusterid: %v, nodeName: %v, metricName: %v", clusterid, nodeName, tc.metricname))
			continue
		}
		maxLen := 0
		maxLenSeries := results[0]
		for i := range results {
			if len(results[i].Samples) > maxLen {
				maxLenSeries = results[i]
			}
		}
		n := len(maxLenSeries.Samples)

		fmt.Println(maxLenSeries.Samples)
		fmt.Println(strconv.FormatFloat(maxLenSeries.Samples[n-1].Value, 'f', 5, 64), maxLenSeries.Samples[n-1].Timestamp)
	}
}
