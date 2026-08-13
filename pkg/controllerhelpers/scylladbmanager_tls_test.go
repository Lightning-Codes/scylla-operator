// Copyright (C) 2026 ScyllaDB

package controllerhelpers

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNewScyllaDBManagerHTTPClientUsesVerifiedMutualTLS(t *testing.T) {
	t.Parallel()

	caCertificate, _ := newTestCertificate(t, true)
	clientCertificate, clientKey := newTestCertificate(t, false)
	clientSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "manager-client", Namespace: "sophena"},
		Data: map[string][]byte{
			"ca.crt":  caCertificate,
			"tls.crt": clientCertificate,
			"tls.key": clientKey,
		},
	}
	kubeClient := fake.NewSimpleClientset(clientSecret)
	smcr := &scyllav1alpha1.ScyllaDBManagerClusterRegistration{
		ObjectMeta: metav1.ObjectMeta{Namespace: "sophena"},
		Spec: scyllav1alpha1.ScyllaDBManagerClusterRegistrationSpec{
			ManagerAPI: scyllav1alpha1.ScyllaDBManagerClusterRegistrationManagerAPI{
				URL: "https://scylla-manager.scylla-manager.svc:5080/api/v1",
				CASecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "manager-client"}, Key: "ca.crt",
				},
				ServerName: "scylla-manager.scylla-manager.svc",
				ClientCertificate: scyllav1alpha1.ScyllaDBManagerClusterRegistrationManagerAPIClientCertificate{
					CertificateSecretKeyRef: corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "manager-client"}, Key: "tls.crt"},
					PrivateKeySecretKeyRef:  corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "manager-client"}, Key: "tls.key"},
				},
			},
		},
	}

	client, err := NewScyllaDBManagerHTTPClient(context.Background(), kubeClient, smcr)
	if err != nil {
		t.Fatalf("can't build verified Manager client: %v", err)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport.TLSClientConfig == nil {
		t.Fatalf("expected HTTP transport with TLS configuration, got %#v", client.Transport)
	}
	tlsConfig := transport.TLSClientConfig
	if tlsConfig.InsecureSkipVerify {
		t.Fatal("Manager API TLS verification must never be disabled")
	}
	if tlsConfig.MinVersion != tls.VersionTLS12 {
		t.Fatalf("expected TLS 1.2 minimum, got %d", tlsConfig.MinVersion)
	}
	if tlsConfig.ServerName != "scylla-manager.scylla-manager.svc" {
		t.Fatalf("unexpected Manager API server name: %q", tlsConfig.ServerName)
	}
	if tlsConfig.RootCAs == nil || len(tlsConfig.Certificates) != 1 {
		t.Fatal("expected explicit Manager RootCAs and one mTLS client certificate")
	}
	if client.Timeout != scyllaDBManagerHTTPTimeout {
		t.Fatalf("expected bounded Manager HTTP timeout %s, got %s", scyllaDBManagerHTTPTimeout, client.Timeout)
	}
}

func TestNewScyllaDBManagerHTTPClientFailsClosedOnInvalidCA(t *testing.T) {
	t.Parallel()

	clientCertificate, clientKey := newTestCertificate(t, false)
	kubeClient := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "manager-client", Namespace: "sophena"},
		Data: map[string][]byte{
			"ca.crt":  []byte("not a PEM CA"),
			"tls.crt": clientCertificate,
			"tls.key": clientKey,
		},
	})
	smcr := &scyllav1alpha1.ScyllaDBManagerClusterRegistration{
		ObjectMeta: metav1.ObjectMeta{Namespace: "sophena"},
		Spec: scyllav1alpha1.ScyllaDBManagerClusterRegistrationSpec{
			ManagerAPI: scyllav1alpha1.ScyllaDBManagerClusterRegistrationManagerAPI{
				URL:            "https://scylla-manager.scylla-manager.svc:5080/api/v1",
				ServerName:     "scylla-manager.scylla-manager.svc",
				CASecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "manager-client"}, Key: "ca.crt"},
				ClientCertificate: scyllav1alpha1.ScyllaDBManagerClusterRegistrationManagerAPIClientCertificate{
					CertificateSecretKeyRef: corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "manager-client"}, Key: "tls.crt"},
					PrivateKeySecretKeyRef:  corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "manager-client"}, Key: "tls.key"},
				},
			},
		},
	}
	if _, err := NewScyllaDBManagerHTTPClient(context.Background(), kubeClient, smcr); err == nil {
		t.Fatal("expected invalid Manager CA to fail closed")
	}
}

func TestNewScyllaDBManagerHTTPClientFailsClosedOnWrongServerName(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	serverCAPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	clientCertificate, clientKey := newTestCertificate(t, false)
	kubeClient := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "manager-client", Namespace: "sophena"},
		Data: map[string][]byte{
			"ca.crt":  serverCAPEM,
			"tls.crt": clientCertificate,
			"tls.key": clientKey,
		},
	})
	smcr := &scyllav1alpha1.ScyllaDBManagerClusterRegistration{
		ObjectMeta: metav1.ObjectMeta{Namespace: "sophena"},
		Spec: scyllav1alpha1.ScyllaDBManagerClusterRegistrationSpec{
			ManagerAPI: scyllav1alpha1.ScyllaDBManagerClusterRegistrationManagerAPI{
				URL:            server.URL,
				ServerName:     "wrong-manager.example.invalid",
				CASecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "manager-client"}, Key: "ca.crt"},
				ClientCertificate: scyllav1alpha1.ScyllaDBManagerClusterRegistrationManagerAPIClientCertificate{
					CertificateSecretKeyRef: corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "manager-client"}, Key: "tls.crt"},
					PrivateKeySecretKeyRef:  corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "manager-client"}, Key: "tls.key"},
				},
			},
		},
	}

	client, err := NewScyllaDBManagerHTTPClient(context.Background(), kubeClient, smcr)
	if err != nil {
		t.Fatalf("can't build Manager HTTP client: %v", err)
	}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("can't build test request: %v", err)
	}
	if _, err := client.Do(request); err == nil {
		t.Fatal("expected the Manager TLS handshake to fail closed on a wrong server name")
	}
}

func newTestCertificate(t *testing.T, isCA bool) ([]byte, []byte) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("can't generate test private key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		BasicConstraintsValid: true,
		IsCA:                  isCA,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}
	if isCA {
		template.KeyUsage |= x509.KeyUsageCertSign
	} else {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	}
	certificateDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("can't create test certificate: %v", err)
	}
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER})
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	return certificatePEM, privateKeyPEM
}
