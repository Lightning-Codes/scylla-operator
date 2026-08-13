// Copyright (C) 2025 ScyllaDB

package controllerhelpers

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/scylladb/scylla-manager/v3/pkg/managerclient"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/managerclientsecure"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	apimachineryutilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
)

const scyllaDBManagerHTTPTimeout = 90 * time.Second

var httpDefaultTransport = http.DefaultTransport.(*http.Transport).Clone()

func NewScyllaDBManagerHTTPClient(
	ctx context.Context,
	kubeClient kubernetes.Interface,
	smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration,
) (*http.Client, error) {
	managerAPIURL, err := url.Parse(smcr.Spec.ManagerAPI.URL)
	if err != nil || managerAPIURL.Scheme != "https" || len(managerAPIURL.Host) == 0 || managerAPIURL.User != nil || managerAPIURL.RawQuery != "" || managerAPIURL.Fragment != "" {
		return nil, fmt.Errorf("Manager API URL must be an absolute https URL without user information, query, or fragment")
	}
	if len(smcr.Spec.ManagerAPI.ServerName) == 0 {
		return nil, fmt.Errorf("Manager API server name must not be empty")
	}

	caBundle, err := GetCAData(ctx, kubeClient, smcr.Namespace, smcr.Spec.ManagerAPI.CAConfigMapKeyRef, smcr.Spec.ManagerAPI.CASecretKeyRef)
	if err != nil {
		return nil, fmt.Errorf("can't get Manager API CA bundle: %w", err)
	}
	rootCAs := x509.NewCertPool()
	if ok := rootCAs.AppendCertsFromPEM(caBundle); !ok {
		return nil, fmt.Errorf("Manager API CA bundle contains no valid PEM certificates")
	}

	certificateSelector := smcr.Spec.ManagerAPI.ClientCertificate.CertificateSecretKeyRef
	privateKeySelector := smcr.Spec.ManagerAPI.ClientCertificate.PrivateKeySecretKeyRef
	certificateSecret, err := kubeClient.CoreV1().Secrets(smcr.Namespace).Get(ctx, certificateSelector.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("can't get Manager API client certificate Secret %q: %w", certificateSelector.Name, err)
	}
	privateKeySecret := certificateSecret
	if privateKeySelector.Name != certificateSelector.Name {
		privateKeySecret, err = kubeClient.CoreV1().Secrets(smcr.Namespace).Get(ctx, privateKeySelector.Name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("can't get Manager API client private key Secret %q: %w", privateKeySelector.Name, err)
		}
	}
	certificatePEM, err := getSecretKey(certificateSecret, certificateSelector.Key)
	if err != nil {
		return nil, fmt.Errorf("can't get Manager API client certificate: %w", err)
	}
	privateKeyPEM, err := getSecretKey(privateKeySecret, privateKeySelector.Key)
	if err != nil {
		return nil, fmt.Errorf("can't get Manager API client private key: %w", err)
	}
	clientCertificate, err := tls.X509KeyPair(certificatePEM, privateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("can't load Manager API client certificate and private key: %w", err)
	}

	transport := httpDefaultTransport.Clone()
	transport.TLSClientConfig = &tls.Config{
		MinVersion:   tls.VersionTLS12,
		RootCAs:      rootCAs,
		ServerName:   smcr.Spec.ManagerAPI.ServerName,
		Certificates: []tls.Certificate{clientCertificate},
	}

	return &http.Client{
		Transport: transport,
		// Secure cluster registration performs verified Agent, CQL, and Alternator
		// handshakes synchronously. Keep this below the controller's reconciliation
		// deadline while allowing the full verified Manager preflight to complete.
		Timeout: scyllaDBManagerHTTPTimeout,
	}, nil
}

func GetCAData(ctx context.Context, kubeClient kubernetes.Interface, namespace string, configMapSelector *corev1.ConfigMapKeySelector, secretSelector *corev1.SecretKeySelector) ([]byte, error) {
	if (configMapSelector == nil) == (secretSelector == nil) {
		return nil, fmt.Errorf("exactly one CA ConfigMap or Secret selector is required")
	}
	if configMapSelector != nil {
		configMap, err := kubeClient.CoreV1().ConfigMaps(namespace).Get(ctx, configMapSelector.Name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("can't get CA ConfigMap %q: %w", configMapSelector.Name, err)
		}
		return getConfigMapKey(configMap, configMapSelector.Key)
	}

	secret, err := kubeClient.CoreV1().Secrets(namespace).Get(ctx, secretSelector.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("can't get CA Secret %q: %w", secretSelector.Name, err)
	}
	return getSecretKey(secret, secretSelector.Key)
}

func GetScyllaDBManagerClient(ctx context.Context, kubeClient kubernetes.Interface, smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration) (*managerclient.Client, error) {
	configuredHTTPClient, err := NewScyllaDBManagerHTTPClient(ctx, kubeClient, smcr)
	if err != nil {
		return nil, fmt.Errorf("can't build verified Manager API HTTP client: %w", err)
	}

	managerClient, err := managerclient.NewClient(smcr.Spec.ManagerAPI.URL, func(httpClient *http.Client) {
		httpClient.Transport = configuredHTTPClient.Transport
		httpClient.Timeout = configuredHTTPClient.Timeout
	})
	if err != nil {
		return nil, fmt.Errorf("can't build manager client: %w", err)
	}

	return &managerClient, nil
}

func GetSecureScyllaDBManagerClient(ctx context.Context, kubeClient kubernetes.Interface, smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration) (*managerclientsecure.Client, error) {
	httpClient, err := NewScyllaDBManagerHTTPClient(ctx, kubeClient, smcr)
	if err != nil {
		return nil, fmt.Errorf("can't build verified Manager API HTTP client: %w", err)
	}

	client, err := managerclientsecure.NewClient(smcr.Spec.ManagerAPI.URL, httpClient)
	if err != nil {
		return nil, fmt.Errorf("can't build secure Manager API client: %w", err)
	}
	return client, nil
}

func getSecretKey(secret *corev1.Secret, key string) ([]byte, error) {
	value, found := secret.Data[key]
	if !found {
		return nil, fmt.Errorf("Secret %q does not contain key %q", secret.Name, key)
	}
	if len(value) == 0 {
		return nil, fmt.Errorf("Secret %q key %q is empty", secret.Name, key)
	}
	return value, nil
}

func getConfigMapKey(configMap *corev1.ConfigMap, key string) ([]byte, error) {
	if value, found := configMap.BinaryData[key]; found {
		if len(value) == 0 {
			return nil, fmt.Errorf("ConfigMap %q key %q is empty", configMap.Name, key)
		}
		return value, nil
	}
	value, found := configMap.Data[key]
	if !found {
		return nil, fmt.Errorf("ConfigMap %q does not contain key %q", configMap.Name, key)
	}
	if len(value) == 0 {
		return nil, fmt.Errorf("ConfigMap %q key %q is empty", configMap.Name, key)
	}
	return []byte(value), nil
}

func IsManagedByGlobalScyllaDBManagerInstance(smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration) bool {
	return naming.GlobalScyllaDBManagerClusterRegistrationSelector().Matches(labels.Set(smcr.GetLabels()))
}

const (
	authTokenSize = 128
)

func newScyllaDBManagerAuthToken() string {
	return apimachineryutilrand.String(authTokenSize)
}

// GetScyllaDBManagerAgentAuthToken retrieves the ScyllaDB Manager agent auth token.
// It greedily gets the auth tokens from the provided functions, returning on the first non-empty result.
// If no auth token is provided by any of the sources, a new auth token is generated.
func GetScyllaDBManagerAgentAuthToken(
	getAuthTokens ...func() ([]metav1.Condition, string, error),
) ([]metav1.Condition, string, error) {
	return getScyllaDBManagerAgentAuthToken(
		newScyllaDBManagerAuthToken,
		getAuthTokens...,
	)
}

func getScyllaDBManagerAgentAuthToken(
	generateAuthToken func() string,
	getAuthTokens ...func() ([]metav1.Condition, string, error),
) ([]metav1.Condition, string, error) {
	var progressingConditions []metav1.Condition
	var authToken string
	var err error

	for _, getAuthToken := range getAuthTokens {
		progressingConditions, authToken, err = getAuthToken()
		if err != nil {
			return progressingConditions, "", fmt.Errorf("can't get ScyllaDB Manager agent auth token: %w", err)
		}
		if len(progressingConditions) > 0 || len(authToken) > 0 {
			// If any of the sources provide a non-empty auth token or progressing conditions, return early.
			return progressingConditions, authToken, nil
		}
	}

	// Generate a new auth token if no source provided one.
	authToken = generateAuthToken()
	return progressingConditions, authToken, nil
}
