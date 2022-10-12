package norm

import (
	"encoding/json"
	"math/rand"
	"os"
	"time"

	simplejson "github.com/bitly/go-simplejson"
	_ "golang.org/x/text/unicode/norm"
	"k8s.io/klog/v2"
)

const (
	NORM_GET_TKE_CREDENTIAL = "NORM.AssumeTkeCredential"

	ENV_CLUSTER_ID = "CLUSTER_ID"
	ENV_APP_ID     = "APPID"
)

// NormRequest
type NormRequest map[string]interface{}

// NormResponse
type NormResponse struct {
	Code     int         `json:"returnValue"`
	Msg      string      `json:"returnMsg"`
	Version  string      `json:"version"`
	Password string      `json:"password"`
	Data     interface{} `json:"returnData"`
}

// NormNoRsp
type NormNoRsp struct {
}

// NormClient
type NormClient struct {
	httpClient
	request  NormRequest
	response NormResponse
}

func getNormUrl() string {
	url := os.Getenv("QCLOUD_NORM_URL")
	if url == "" {
		url = "http://169.254.0.40:80/norm/api"
	}
	return url
}

// NewNormClient
func NewNormClient() *NormClient {
	c := &NormClient{
		request:  NormRequest{},
		response: NormResponse{},
	}
	c.httpClient = httpClient{
		url:     getNormUrl(),
		timeout: 10,
		caller:  "crane",
		callee:  "NORM",
		packer:  c,
	}
	return c
}

// 从环境变量中获取clusterID和appId放入norm的para中 for meta clustercache
func SetNormReqExt(body []byte) []byte {
	js, err := simplejson.NewJson(body)
	if err != nil {
		klog.Error("SetNormReqExt NewJson error,", err)
		return nil
	}
	if os.Getenv(ENV_CLUSTER_ID) != "" {
		js.SetPath([]string{"provider", "para", "unClusterId"}, os.Getenv(ENV_CLUSTER_ID))
		klog.V(6).Info("SetNormReqExt set unClusterId", os.Getenv(ENV_CLUSTER_ID))

	}
	if os.Getenv(ENV_APP_ID) != "" {
		js.SetPath([]string{"provider", "para", "appId"}, os.Getenv(ENV_APP_ID))
		klog.V(6).Info("SetNormReqExt set appId", os.Getenv(ENV_APP_ID))

	}

	out, err := js.Encode()
	if err != nil {
		klog.Error("SetNormReqExt Encode error,", err)
		return body
	}

	return out
}

func (c *NormClient) packRequest(interfaceName string, reqObj interface{}) ([]byte, error) {
	c.request = map[string]interface{}{
		"eventId":   rand.Uint32(),
		"timestamp": time.Now().Unix(),
		"caller":    c.httpClient.caller,
		"callee":    c.httpClient.callee,
		"version":   "1",
		"password":  "cloudprovider",
		"interface": map[string]interface{}{
			"interfaceName": interfaceName,
			"para":          reqObj,
		},
	}

	b, err := json.Marshal(c.request)
	if err != nil {
		klog.Error("packRequest failed:", err, ", req:", c.request)
		return nil, err
	}
	b = SetNormReqExt(b)
	return b, nil
}

func (c *NormClient) unPackResponse(data []byte, responseData interface{}) (err error) {
	c.response.Data = responseData
	err = json.Unmarshal(data, &c.response)
	if err != nil {
		klog.Error("Unmarshal goods response err:", err)
		return

	}
	return
}

func (c *NormClient) getResult() (int, string) {
	return c.response.Code, c.response.Msg
}

// NormGetTkeCredentialReq
type NormGetTkeCredentialReq struct {
	Duration  int    `json:"duration"`
	ClusterId string `json:"unClusterId"`
	AppId     string `json:"appId"`
}

// CredentialData
type CredentialData struct {
	Credentials struct {
		SessionToken string `json:"sessionToken"`
		TmpSecretId  string `json:"tmpSecretId"`
		TmpSecretKey string `json:"tmpSecretKey"`
	} `json:"credentials"`
	ExpiredTime int `json:"expiredTime"`
}

// NormGetTkeCredential
func NormGetTkeCredential(req NormGetTkeCredentialReq) (*CredentialData, error) {
	c := NewNormClient()
	rsp := &CredentialData{}
	err := c.DoRequest(NORM_GET_TKE_CREDENTIAL, req, rsp)
	if err != nil {
		//if normError, ok := err.(RequestResultError); ok {
		//	if normError.Code == -8002 {
		//		return nil, serviceError.NewServiceError(serviceError.NormTKEQCSRoleNotExistError, normError.Error())
		//	} else if normError.Code == -8017 {
		//		return nil, serviceError.NewServiceError(serviceError.NormIpMasqAgentConfigChangedError, normError.Error())
		//	}
		//	return nil, serviceError.NewServiceError(serviceError.ServiceNormError, normError.Error())
		//}
		klog.Errorf("norm get tke credential err: %s\n", err)
		return nil, err
	}
	return rsp, nil
}
