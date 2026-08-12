// Copyright (C) 2025 ScyllaDB

package validation

import (
	"net"
	"net/url"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var (
	scyllaDBManagerClusterRegistrationSupportedLocalScyllaDBReferenceKinds = []string{
		scyllav1.ScyllaClusterGVK.Kind,
		scyllav1alpha1.ScyllaDBDatacenterGVK.Kind,
		scyllav1alpha1.ScyllaDBClusterGVK.Kind,
	}
)

func ValidateScyllaDBManagerClusterRegistration(smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, ValidateScyllaDBManagerClusterRegistrationObjectMeta(&smcr.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateScyllaDBManagerClusterRegistrationSpec(&smcr.Spec, field.NewPath("spec"))...)

	return allErrs
}

func ValidateScyllaDBManagerClusterRegistrationObjectMeta(objectMeta *metav1.ObjectMeta, fldPath *field.Path) field.ErrorList {
	// ScyllaDBManagerClusterRegistration is a supported user-owned API. The global Manager
	// controller's label is therefore optional and only identifies controller-owned registrations.
	return nil
}

func ValidateScyllaDBManagerClusterRegistrationSpec(spec *scyllav1alpha1.ScyllaDBManagerClusterRegistrationSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, ValidateLocalScyllaDBReference(&spec.ScyllaDBClusterRef, scyllaDBManagerClusterRegistrationSupportedLocalScyllaDBReferenceKinds, fldPath.Child("scyllaDBClusterRef"))...)
	allErrs = append(allErrs, validateScyllaDBManagerClusterRegistrationManagerAPI(&spec.ManagerAPI, fldPath.Child("managerAPI"))...)

	if spec.Authentication == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("authentication"), "CQL authentication is required"))
	} else {
		if spec.Authentication.CQL == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("authentication", "cql"), "CQL authentication is required"))
		} else {
			allErrs = append(allErrs, validateRequiredSecretKeySelector(&spec.Authentication.CQL.UsernameSecretKeyRef, fldPath.Child("authentication", "cql", "usernameSecretKeyRef"))...)
			allErrs = append(allErrs, validateRequiredSecretKeySelector(&spec.Authentication.CQL.PasswordSecretKeyRef, fldPath.Child("authentication", "cql", "passwordSecretKeyRef"))...)
		}
		if spec.Authentication.Alternator != nil {
			allErrs = append(allErrs, validateRequiredSecretKeySelector(&spec.Authentication.Alternator.AccessKeyIDSecretKeyRef, fldPath.Child("authentication", "alternator", "accessKeyIDSecretKeyRef"))...)
			allErrs = append(allErrs, validateRequiredSecretKeySelector(&spec.Authentication.Alternator.SecretAccessKeySecretKeyRef, fldPath.Child("authentication", "alternator", "secretAccessKeySecretKeyRef"))...)
		}
	}

	if spec.TLS == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("tls"), "CQL and Agent TLS are required"))
	} else {
		if spec.TLS.CQL == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("tls", "cql"), "CQL TLS verification is required"))
		} else {
			allErrs = append(allErrs, validateScyllaDBManagerClusterRegistrationTLSConfig(spec.TLS.CQL, fldPath.Child("tls", "cql"))...)
		}
		if spec.TLS.Alternator != nil {
			allErrs = append(allErrs, validateScyllaDBManagerClusterRegistrationTLSConfig(spec.TLS.Alternator, fldPath.Child("tls", "alternator"))...)
		}
		if spec.TLS.Agent == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("tls", "agent"), "Agent TLS verification is required"))
		} else {
			allErrs = append(allErrs, validateScyllaDBManagerClusterRegistrationTLSConfig(spec.TLS.Agent, fldPath.Child("tls", "agent"))...)
		}
	}

	hasAlternatorAuthentication := spec.Authentication != nil && spec.Authentication.Alternator != nil
	hasAlternatorTLS := spec.TLS != nil && spec.TLS.Alternator != nil
	if hasAlternatorAuthentication != hasAlternatorTLS {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("authentication", "alternator"), hasAlternatorAuthentication, "Alternator authentication and TLS must be configured together"))
	}

	return allErrs
}

func validateScyllaDBManagerClusterRegistrationManagerAPI(config *scyllav1alpha1.ScyllaDBManagerClusterRegistrationManagerAPI, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	managerURL, err := url.Parse(config.URL)
	if len(config.URL) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("url"), ""))
	} else if err != nil || managerURL.Scheme != "https" || len(managerURL.Host) == 0 || managerURL.User != nil || managerURL.RawQuery != "" || managerURL.Fragment != "" {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("url"), config.URL, "must be an absolute https URL without user information, query, or fragment"))
	}

	allErrs = append(allErrs, validateCASelector(config.CAConfigMapKeyRef, config.CASecretKeyRef, fldPath)...)
	allErrs = append(allErrs, validateServerName(config.ServerName, fldPath.Child("serverName"))...)
	allErrs = append(allErrs, validateRequiredSecretKeySelector(&config.ClientCertificate.CertificateSecretKeyRef, fldPath.Child("clientCertificate", "certificateSecretKeyRef"))...)
	allErrs = append(allErrs, validateRequiredSecretKeySelector(&config.ClientCertificate.PrivateKeySecretKeyRef, fldPath.Child("clientCertificate", "privateKeySecretKeyRef"))...)

	return allErrs
}

func validateRequiredSecretKeySelector(selector *corev1.SecretKeySelector, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, validateRequiredLocalObjectName(selector.Name, fldPath.Child("name"))...)
	allErrs = append(allErrs, validateRequiredDataKey(selector.Key, fldPath.Child("key"))...)
	if selector.Optional != nil && *selector.Optional {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("optional"), *selector.Optional, "must be false because connection material is required"))
	}

	return allErrs
}

func validateRequiredConfigMapKeySelector(selector *corev1.ConfigMapKeySelector, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, validateRequiredLocalObjectName(selector.Name, fldPath.Child("name"))...)
	allErrs = append(allErrs, validateRequiredDataKey(selector.Key, fldPath.Child("key"))...)
	if selector.Optional != nil && *selector.Optional {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("optional"), *selector.Optional, "must be false because the CA bundle is required"))
	}

	return allErrs
}

func validateCASelector(configMapSelector *corev1.ConfigMapKeySelector, secretSelector *corev1.SecretKeySelector, fldPath *field.Path) field.ErrorList {
	if configMapSelector == nil && secretSelector == nil {
		return field.ErrorList{field.Required(fldPath, "exactly one of caConfigMapKeyRef or caSecretKeyRef is required")}
	}
	if configMapSelector != nil && secretSelector != nil {
		return field.ErrorList{field.Invalid(fldPath, "multiple CA selectors", "exactly one of caConfigMapKeyRef or caSecretKeyRef may be set")}
	}
	if configMapSelector != nil {
		return validateRequiredConfigMapKeySelector(configMapSelector, fldPath.Child("caConfigMapKeyRef"))
	}
	return validateRequiredSecretKeySelector(secretSelector, fldPath.Child("caSecretKeyRef"))
}

func validateRequiredLocalObjectName(name string, fldPath *field.Path) field.ErrorList {
	if len(name) == 0 {
		return field.ErrorList{field.Required(fldPath, "")}
	}

	var allErrs field.ErrorList
	for _, msg := range apimachineryutilvalidation.IsDNS1123Subdomain(name) {
		allErrs = append(allErrs, field.Invalid(fldPath, name, msg))
	}
	return allErrs
}

func validateRequiredDataKey(key string, fldPath *field.Path) field.ErrorList {
	if len(key) == 0 {
		return field.ErrorList{field.Required(fldPath, "")}
	}

	var allErrs field.ErrorList
	for _, msg := range apimachineryutilvalidation.IsConfigMapKey(key) {
		allErrs = append(allErrs, field.Invalid(fldPath, key, msg))
	}
	return allErrs
}

func validateScyllaDBManagerClusterRegistrationTLSConfig(config *scyllav1alpha1.ScyllaDBManagerClusterRegistrationTLSConfig, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, validateCASelector(config.CAConfigMapKeyRef, config.CASecretKeyRef, fldPath)...)
	allErrs = append(allErrs, validateServerName(config.ServerName, fldPath.Child("serverName"))...)

	return allErrs
}

func validateServerName(serverName string, fldPath *field.Path) field.ErrorList {
	if len(serverName) == 0 {
		return field.ErrorList{field.Required(fldPath, "")}
	}
	if net.ParseIP(serverName) != nil {
		return nil
	}

	var allErrs field.ErrorList
	for _, msg := range apimachineryutilvalidation.IsDNS1123Subdomain(serverName) {
		allErrs = append(allErrs, field.Invalid(fldPath, serverName, msg))
	}
	return allErrs
}

func ValidateLocalScyllaDBReference(localScyllaDBReference *scyllav1alpha1.LocalScyllaDBReference, supportedLocalScyllaDBReferenceKinds []string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if len(localScyllaDBReference.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), ""))
	} else {
		for _, msg := range apimachineryutilvalidation.IsDNS1123Subdomain(localScyllaDBReference.Name) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), localScyllaDBReference.Name, msg))
		}
	}

	if len(localScyllaDBReference.Kind) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("kind"), ""))
	} else {
		allErrs = append(allErrs, validateEnum(localScyllaDBReference.Kind, supportedLocalScyllaDBReferenceKinds, fldPath.Child("kind"))...)
	}

	return allErrs
}

func ValidateScyllaDBManagerClusterRegistrationUpdate(new, old *scyllav1alpha1.ScyllaDBManagerClusterRegistration) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, ValidateScyllaDBManagerClusterRegistration(new)...)
	allErrs = append(allErrs, ValidateScyllaDBManagerClusterRegistrationObjectMetaUpdate(&new.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateScyllaDBManagerClusterRegistrationSpecUpdate(&new.Spec, &old.Spec, field.NewPath("spec"))...)

	return allErrs
}

func ValidateScyllaDBManagerClusterRegistrationObjectMetaUpdate(newObjectMeta, oldObjectMeta *metav1.ObjectMeta, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, apimachineryvalidation.ValidateImmutableField(newObjectMeta.Annotations[naming.ScyllaDBManagerClusterRegistrationNameOverrideAnnotation], oldObjectMeta.Annotations[naming.ScyllaDBManagerClusterRegistrationNameOverrideAnnotation], fldPath.Child("annotations").Key(naming.ScyllaDBManagerClusterRegistrationNameOverrideAnnotation))...)

	return allErrs
}

func ValidateScyllaDBManagerClusterRegistrationSpecUpdate(newSpec, oldSpec *scyllav1alpha1.ScyllaDBManagerClusterRegistrationSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, apimachineryvalidation.ValidateImmutableField(newSpec.ScyllaDBClusterRef.Kind, oldSpec.ScyllaDBClusterRef.Kind, fldPath.Child("scyllaDBClusterRef", "kind"))...)
	allErrs = append(allErrs, apimachineryvalidation.ValidateImmutableField(newSpec.ScyllaDBClusterRef.Name, oldSpec.ScyllaDBClusterRef.Name, fldPath.Child("scyllaDBClusterRef", "name"))...)

	return allErrs
}

func GetWarningsOnScyllaDBManagerClusterRegistrationCreate(smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration) []string {
	return nil
}

func GetWarningsOnScyllaDBManagerClusterRegistrationUpdate(new, old *scyllav1alpha1.ScyllaDBManagerClusterRegistration) []string {
	return nil
}
