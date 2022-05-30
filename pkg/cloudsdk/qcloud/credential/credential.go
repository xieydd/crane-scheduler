package credential

import (
	"sync"
	"time"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
)

type QCloudCredential interface {
	GetQCloudCredential() (*common.Credential, error)
	UpdateQCloudCustomCredential(secretId, secretKey string) *common.Credential

	// for test
	UpdateQCloudAssumedCredential(secretId, secretKey, token string) *common.Credential
	DisableRefresh()
}

// CustomCredential use user defined SecretId and SecretKey
type CustomCredential struct {
	lock              sync.Mutex
	customedSecretId  string
	customedSecretKey string
}

func (cred *CustomCredential) DisableRefresh() {
	panic("implement me")
}

func (cred *CustomCredential) UpdateQCloudAssumedCredential(secretId, secretKey, token string) *common.Credential {
	panic("implement me")
}

func NewQCloudCredential(clusterId, appId, secretId, secretKey string, expiredDuration time.Duration) QCloudCredential {
	return &CustomCredential{
		customedSecretId:  secretId,
		customedSecretKey: secretKey,
	}
}

func (cred *CustomCredential) GetQCloudCustomCredential() *common.Credential {
	cred.lock.Lock()
	defer cred.lock.Unlock()

	credential := &common.Credential{}

	credential.SecretId = cred.customedSecretId
	credential.SecretKey = cred.customedSecretKey

	return credential
}

func (cred *CustomCredential) GetQCloudCredential() (*common.Credential, error) {
	cred.lock.Lock()
	defer cred.lock.Unlock()

	return &common.Credential{
		SecretId:  cred.customedSecretId,
		SecretKey: cred.customedSecretKey,
	}, nil
}

func (cred *CustomCredential) UpdateQCloudCustomCredential(secretId, secretKey string) *common.Credential {
	cred.lock.Lock()
	defer cred.lock.Unlock()

	cred.customedSecretId = secretId
	cred.customedSecretKey = secretKey
	return &common.Credential{SecretId: cred.customedSecretId, SecretKey: cred.customedSecretKey}
}
