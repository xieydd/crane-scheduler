package credential

import (
	"sync"
	"time"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"

	"github.com/gocrane/crane-scheduler/pkg/cloudsdk/qcloud/norm"
)

type Refresher interface {
	Refresh() (string, string, string, int, error)
}

type NormRefresher struct {
	expiredDuration time.Duration
	clusterId       string
	appId           string
}

func NewNormRefresher(expiredDuration time.Duration, clusterId, appId string) (NormRefresher, error) {
	return NormRefresher{
		expiredDuration: expiredDuration,
		clusterId:       clusterId,
		appId:           appId,
	}, nil
}

func (nr NormRefresher) Refresh() (secretId string, secretKey string, token string, expiredAt int, err error) {
	var rsp *norm.CredentialData
	rsp, err = norm.NormGetTkeCredential(
		norm.NormGetTkeCredentialReq{
			ClusterId: nr.clusterId,
			AppId:     nr.appId,
			Duration:  int(nr.expiredDuration / time.Second),
		},
	)
	if err != nil {
		return "", "", "", 0, err
	}
	secretId = rsp.Credentials.TmpSecretId
	secretKey = rsp.Credentials.TmpSecretKey
	token = rsp.Credentials.SessionToken
	expiredAt = rsp.ExpiredTime

	return
}

type normCredential struct {
	// assumed id and key, it is temporary
	SecretId  string
	SecretKey string
	Token     string

	expiredAt       time.Time
	expiredDuration time.Duration // TODO: maybe confused with this? find a better way

	lock sync.Mutex

	refresher Refresher
}

func (normCred *normCredential) needRefresh() bool {
	return time.Now().Add(normCred.expiredDuration / time.Second / 2 * time.Second).After(normCred.expiredAt)
}

func (normCred *normCredential) refresh() error {
	secretId, secretKey, token, expiredAt, err := normCred.refresher.Refresh()
	if err != nil {
		return err
	}

	normCred.updateExpiredAt(time.Unix(int64(expiredAt), 0))
	normCred.updateCredential(secretId, secretKey, token)

	return nil
}

func (normCred *normCredential) updateExpiredAt(expiredAt time.Time) {
	normCred.lock.Lock()
	defer normCred.lock.Unlock()

	normCred.expiredAt = expiredAt
}

func (normCred *normCredential) updateCredential(secretId, secretKey, token string) {
	normCred.lock.Lock()
	defer normCred.lock.Unlock()

	normCred.SecretId = secretId
	normCred.SecretKey = secretKey
	normCred.Token = token
}

func (normCred *normCredential) getCredential() *common.Credential {
	normCred.lock.Lock()
	defer normCred.lock.Unlock()
	return common.NewTokenCredential(normCred.SecretId, normCred.SecretKey, normCred.Token)
}

type credential struct {
	normCred *normCredential

	lock              sync.Mutex
	customedSecretId  string
	customedSecretKey string
	// test only
	disableRefresh bool
}

func NewQCloudNormCredential(clusterId, appId, secretId, secretKey string, expiredDuration time.Duration) QCloudCredential {
	refresher, _ := NewNormRefresher(expiredDuration, clusterId, appId)
	return &credential{
		normCred: &normCredential{
			expiredDuration: expiredDuration,
			refresher:       refresher,
		},
		customedSecretId:  secretId,
		customedSecretKey: secretKey,
	}
}

func (cred *credential) GetQCloudCredential() (*common.Credential, error) {
	return cred.GetQCloudAssumedCredential()
}

func (cred *credential) GetQCloudAssumedCredential() (*common.Credential, error) {
	credential := common.Credential{}
	if !cred.disableRefresh && cred.normCred.needRefresh() {
		if err := cred.normCred.refresh(); err != nil {
			return &credential, err
		}
	}
	return cred.normCred.getCredential(), nil
}

func (cred *credential) DisableRefresh() {
	cred.disableRefresh = true
}

func (cred *credential) GetQCloudCustomCredential() (*common.Credential, error) {
	cred.lock.Lock()
	defer cred.lock.Unlock()

	credential := &common.Credential{}

	credential.SecretId = cred.customedSecretId
	credential.SecretKey = cred.customedSecretKey

	return credential, nil
}

func (cred *credential) UpdateQCloudCustomCredential(secretId, secretKey string) *common.Credential {
	cred.lock.Lock()
	defer cred.lock.Unlock()

	cred.customedSecretId = secretId
	cred.customedSecretKey = secretKey
	return &common.Credential{SecretId: cred.customedSecretId, SecretKey: cred.customedSecretKey}
}

func (cred *credential) UpdateQCloudAssumedCredential(secretId, secretKey, token string) *common.Credential {
	cred.normCred.updateCredential(secretId, secretKey, token)
	return cred.normCred.getCredential()
}

func (cred *credential) UpdateQCloudAssumedCredentialExpiredAt(expiredAt time.Time) {
	cred.normCred.updateExpiredAt(expiredAt)
}
