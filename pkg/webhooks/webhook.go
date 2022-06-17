package webhooks

import (
	"context"
	"encoding/json"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gocrane/crane-scheduler/pkg/known"
	"github.com/gocrane/crane-scheduler/pkg/utils"
)

type Config struct {
	Enabled     bool
	HookPort    int
	HookHost    string
	HookCertDir string
}

type PodMutate struct {
	Client  client.Client
	decoder *admission.Decoder
}

func (p *PodMutate) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}

	err := p.decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	klog.V(6).Infof("webhook mutating pod: %v", klog.KObj(pod))

	cm := corev1.ConfigMap{}
	err = p.Client.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceSystem, Name: known.ConfigMapSchedulerApplyScope}, &cm)
	if err == nil || errors.IsNotFound(err) {
		namespacesScope := utils.GetSchedulerNamespaceApplyScope(&cm)
		clusterScope := namespacesScope[known.WildCard]
		// use AdmissionRequest namespace instead object namespace because of object maybe has no namespace and name.
		// this is because for some no namespace object in yaml, apiserver registry fill the namespace after the mutating webhook
		podApply := namespacesScope[req.Namespace]
		if clusterScope || podApply {
			ann := pod.GetAnnotations()
			if ann == nil {
				ann = make(map[string]string)
			}
			// this is used for scheduler to decide to schedule on housekeeper, for crane housekeeper extender scheduler to do some differentiation capability with normal nodes
			ann[known.AnnotationPodSchedulingScope] = known.AnnotationPodSchedulingScopeHousekeeperVal
			pod.Annotations = ann
			nodeSelector := pod.Spec.NodeSelector
			if nodeSelector == nil {
				nodeSelector = make(map[string]string)
			}
			nodeSelector[known.LabelHousekeeperNodeKey] = known.LabelHousekeeperNodeVal
			pod.Spec.NodeSelector = nodeSelector
		}
	} else {
		klog.Error(err, "failed get configmap config")
		return admission.Errored(http.StatusInternalServerError, err)
	}

	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		klog.Error(err, "failed marshal pod")
		return admission.Errored(http.StatusInternalServerError, err)
	}

	klog.V(6).Infof("webhook mutated pod: %s", marshaledPod)

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

// podMutate implements admission.DecoderInjector.
// A decoder will be automatically injected.

// InjectDecoder injects the decoder.
func (p *PodMutate) InjectDecoder(d *admission.Decoder) error {
	p.decoder = d
	return nil
}
