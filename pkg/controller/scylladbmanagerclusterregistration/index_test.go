// Copyright (C) 2026 ScyllaDB

package scylladbmanagerclusterregistration

import (
	"reflect"
	"testing"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConnectionMaterialIndexes(t *testing.T) {
	t.Parallel()

	smcr := &scyllav1alpha1.ScyllaDBManagerClusterRegistration{
		ObjectMeta: metav1.ObjectMeta{Name: "scyllacluster-scylladb", Namespace: "sophena"},
		Spec: scyllav1alpha1.ScyllaDBManagerClusterRegistrationSpec{
			ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{Kind: scyllav1.ScyllaClusterGVK.Kind, Name: "scylladb"},
			ManagerAPI: scyllav1alpha1.ScyllaDBManagerClusterRegistrationManagerAPI{
				CASecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "manager-client"}, Key: "ca.crt"},
				ClientCertificate: scyllav1alpha1.ScyllaDBManagerClusterRegistrationManagerAPIClientCertificate{
					CertificateSecretKeyRef: corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "manager-client"}, Key: "tls.crt"},
					PrivateKeySecretKeyRef:  corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "manager-client"}, Key: "tls.key"},
				},
			},
			Authentication: &scyllav1alpha1.ScyllaDBManagerClusterRegistrationAuthentication{
				CQL: &scyllav1alpha1.ScyllaDBManagerClusterRegistrationCQLAuthentication{
					UsernameSecretKeyRef: corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "database-auth"}, Key: "username"},
					PasswordSecretKeyRef: corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "database-auth"}, Key: "password"},
				},
			},
			TLS: &scyllav1alpha1.ScyllaDBManagerClusterRegistrationTLS{
				CQL: &scyllav1alpha1.ScyllaDBManagerClusterRegistrationTLSConfig{
					CAConfigMapKeyRef: &corev1.ConfigMapKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "database-ca"}, Key: "ca-bundle.crt"},
					ClientCertificate: &scyllav1alpha1.ScyllaDBManagerClusterRegistrationTLSClientCertificate{
						CertificateSecretKeyRef: corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "database-client-tls"}, Key: corev1.TLSCertKey},
						PrivateKeySecretKeyRef:  corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "database-client-tls"}, Key: corev1.TLSPrivateKeyKey},
					},
				},
				Agent: &scyllav1alpha1.ScyllaDBManagerClusterRegistrationTLSConfig{
					CASecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "agent-ca"}, Key: "ca.crt"},
				},
			},
		},
	}

	secretRefs, err := indexScyllaDBManagerClusterRegistrationBySecret(smcr)
	if err != nil {
		t.Fatalf("can't index Secret references: %v", err)
	}
	expectedSecretRefs := []string{
		"sophena/agent-ca",
		"sophena/database-auth",
		"sophena/database-client-tls",
		"sophena/manager-client",
		"sophena/scylladb-auth-token",
	}
	if !reflect.DeepEqual(secretRefs, expectedSecretRefs) {
		t.Fatalf("unexpected Secret references: expected %v, got %v", expectedSecretRefs, secretRefs)
	}

	configMapRefs, err := indexScyllaDBManagerClusterRegistrationByConfigMap(smcr)
	if err != nil {
		t.Fatalf("can't index ConfigMap references: %v", err)
	}
	expectedConfigMapRefs := []string{"sophena/database-ca"}
	if !reflect.DeepEqual(configMapRefs, expectedConfigMapRefs) {
		t.Fatalf("unexpected ConfigMap references: expected %v, got %v", expectedConfigMapRefs, configMapRefs)
	}

	localRefs, err := indexScyllaDBManagerClusterRegistrationByLocalRef(smcr)
	if err != nil {
		t.Fatalf("can't index local ScyllaDB reference: %v", err)
	}
	expectedLocalRefs := []string{"sophena/ScyllaCluster/scylladb"}
	if !reflect.DeepEqual(localRefs, expectedLocalRefs) {
		t.Fatalf("unexpected local ScyllaDB references: expected %v, got %v", expectedLocalRefs, localRefs)
	}
}
