// Copyright (C) 2026 ScyllaDB

package scylladbmanagerclusterregistration

import (
	"context"
	"strings"
	"testing"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestFinalizerFailsClosedWithoutVerifiedManagerAPICA(t *testing.T) {
	t.Parallel()

	smcr := &scyllav1alpha1.ScyllaDBManagerClusterRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "secure-registration",
			Namespace:  "sophena",
			Finalizers: []string{naming.ScyllaDBManagerClusterRegistrationFinalizer},
			Labels: map[string]string{
				naming.GlobalScyllaDBManagerLabel: naming.LabelValueTrue,
			},
		},
		Spec: scyllav1alpha1.ScyllaDBManagerClusterRegistrationSpec{
			ManagerAPI: scyllav1alpha1.ScyllaDBManagerClusterRegistrationManagerAPI{
				URL:            "https://scylla-manager.scylla-manager.svc/api/v1",
				ServerName:     "scylla-manager.scylla-manager.svc",
				CASecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "manager-client"}, Key: "ca.crt"},
				ClientCertificate: scyllav1alpha1.ScyllaDBManagerClusterRegistrationManagerAPIClientCertificate{
					CertificateSecretKeyRef: corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "manager-client"}, Key: "tls.crt"},
					PrivateKeySecretKeyRef:  corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "manager-client"}, Key: "tls.key"},
				},
			},
		},
	}

	kubeClient := fake.NewSimpleClientset()
	controller := &Controller{kubeClient: kubeClient}
	_, err := controller.syncFinalizer(context.Background(), smcr)
	if err == nil || !strings.Contains(err.Error(), "can't get Manager API CA bundle") {
		t.Fatalf("expected finalization to fail closed on the missing verified Manager API CA, got %v", err)
	}

	actions := kubeClient.Actions()
	if len(actions) != 1 || actions[0].GetVerb() != "get" || actions[0].GetResource().Resource != "secrets" {
		t.Fatalf("expected only the verified Manager CA lookup before finalization stopped, got %#v", actions)
	}
}
