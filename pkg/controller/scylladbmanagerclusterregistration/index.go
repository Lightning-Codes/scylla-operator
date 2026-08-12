// Copyright (C) 2026 ScyllaDB

package scylladbmanagerclusterregistration

import (
	"fmt"
	"sort"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	scyllaDBManagerClusterRegistrationBySecretIndexName    = "connection-secret"
	scyllaDBManagerClusterRegistrationByConfigMapIndexName = "connection-configmap"
	scyllaDBManagerClusterRegistrationByLocalRefIndexName  = "local-scylladb-reference"
)

func namespacedReferenceKey(namespace, name string) string {
	return namespace + "/" + name
}

func localScyllaDBReferenceKey(namespace, kind, name string) string {
	return namespace + "/" + kind + "/" + name
}

func indexScyllaDBManagerClusterRegistrationByLocalRef(obj interface{}) ([]string, error) {
	smcr, ok := obj.(*scyllav1alpha1.ScyllaDBManagerClusterRegistration)
	if !ok {
		return nil, fmt.Errorf("expected *scyllav1alpha1.ScyllaDBManagerClusterRegistration, got %T", obj)
	}

	return []string{localScyllaDBReferenceKey(smcr.Namespace, smcr.Spec.ScyllaDBClusterRef.Kind, smcr.Spec.ScyllaDBClusterRef.Name)}, nil
}

func indexScyllaDBManagerClusterRegistrationBySecret(obj interface{}) ([]string, error) {
	smcr, ok := obj.(*scyllav1alpha1.ScyllaDBManagerClusterRegistration)
	if !ok {
		return nil, fmt.Errorf("expected *scyllav1alpha1.ScyllaDBManagerClusterRegistration, got %T", obj)
	}

	refs := map[string]struct{}{}
	switch smcr.Spec.ScyllaDBClusterRef.Kind {
	case scyllav1.ScyllaClusterGVK.Kind:
		refs[namespacedReferenceKey(smcr.Namespace, naming.AgentAuthTokenSecretNameForScyllaCluster(&scyllav1.ScyllaCluster{
			ObjectMeta: metav1.ObjectMeta{Name: smcr.Spec.ScyllaDBClusterRef.Name},
		}))] = struct{}{}
	case scyllav1alpha1.ScyllaDBDatacenterGVK.Kind:
		refs[namespacedReferenceKey(smcr.Namespace, naming.AgentAuthTokenSecretName(&scyllav1alpha1.ScyllaDBDatacenter{
			ObjectMeta: metav1.ObjectMeta{Name: smcr.Spec.ScyllaDBClusterRef.Name},
		}))] = struct{}{}
	case scyllav1alpha1.ScyllaDBClusterGVK.Kind:
		secretName, err := naming.ScyllaDBManagerAgentAuthTokenSecretNameForScyllaDBCluster(&scyllav1alpha1.ScyllaDBCluster{
			ObjectMeta: metav1.ObjectMeta{Name: smcr.Spec.ScyllaDBClusterRef.Name},
		})
		if err != nil {
			return nil, fmt.Errorf("can't calculate ScyllaDBCluster Agent auth token Secret name: %w", err)
		}
		refs[namespacedReferenceKey(smcr.Namespace, secretName)] = struct{}{}
	}
	if smcr.Spec.ManagerAPI.CASecretKeyRef != nil {
		refs[namespacedReferenceKey(smcr.Namespace, smcr.Spec.ManagerAPI.CASecretKeyRef.Name)] = struct{}{}
	}
	refs[namespacedReferenceKey(smcr.Namespace, smcr.Spec.ManagerAPI.ClientCertificate.CertificateSecretKeyRef.Name)] = struct{}{}
	refs[namespacedReferenceKey(smcr.Namespace, smcr.Spec.ManagerAPI.ClientCertificate.PrivateKeySecretKeyRef.Name)] = struct{}{}
	if smcr.Spec.Authentication != nil {
		if smcr.Spec.Authentication.CQL != nil {
			refs[namespacedReferenceKey(smcr.Namespace, smcr.Spec.Authentication.CQL.UsernameSecretKeyRef.Name)] = struct{}{}
			refs[namespacedReferenceKey(smcr.Namespace, smcr.Spec.Authentication.CQL.PasswordSecretKeyRef.Name)] = struct{}{}
		}
		if smcr.Spec.Authentication.Alternator != nil {
			refs[namespacedReferenceKey(smcr.Namespace, smcr.Spec.Authentication.Alternator.AccessKeyIDSecretKeyRef.Name)] = struct{}{}
			refs[namespacedReferenceKey(smcr.Namespace, smcr.Spec.Authentication.Alternator.SecretAccessKeySecretKeyRef.Name)] = struct{}{}
		}
	}
	if smcr.Spec.TLS != nil {
		for _, tlsConfig := range []*scyllav1alpha1.ScyllaDBManagerClusterRegistrationTLSConfig{smcr.Spec.TLS.CQL, smcr.Spec.TLS.Alternator, smcr.Spec.TLS.Agent} {
			if tlsConfig != nil && tlsConfig.CASecretKeyRef != nil {
				refs[namespacedReferenceKey(smcr.Namespace, tlsConfig.CASecretKeyRef.Name)] = struct{}{}
			}
			if tlsConfig != nil && tlsConfig.ClientCertificate != nil {
				refs[namespacedReferenceKey(smcr.Namespace, tlsConfig.ClientCertificate.CertificateSecretKeyRef.Name)] = struct{}{}
				refs[namespacedReferenceKey(smcr.Namespace, tlsConfig.ClientCertificate.PrivateKeySecretKeyRef.Name)] = struct{}{}
			}
		}
	}

	return keys(refs), nil
}

func indexScyllaDBManagerClusterRegistrationByConfigMap(obj interface{}) ([]string, error) {
	smcr, ok := obj.(*scyllav1alpha1.ScyllaDBManagerClusterRegistration)
	if !ok {
		return nil, fmt.Errorf("expected *scyllav1alpha1.ScyllaDBManagerClusterRegistration, got %T", obj)
	}

	refs := map[string]struct{}{}
	if smcr.Spec.ManagerAPI.CAConfigMapKeyRef != nil {
		refs[namespacedReferenceKey(smcr.Namespace, smcr.Spec.ManagerAPI.CAConfigMapKeyRef.Name)] = struct{}{}
	}
	if smcr.Spec.TLS != nil {
		for _, tlsConfig := range []*scyllav1alpha1.ScyllaDBManagerClusterRegistrationTLSConfig{smcr.Spec.TLS.CQL, smcr.Spec.TLS.Alternator, smcr.Spec.TLS.Agent} {
			if tlsConfig != nil && tlsConfig.CAConfigMapKeyRef != nil {
				refs[namespacedReferenceKey(smcr.Namespace, tlsConfig.CAConfigMapKeyRef.Name)] = struct{}{}
			}
		}
	}

	return keys(refs), nil
}

func keys(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
