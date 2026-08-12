// Copyright (C) 2025 ScyllaDB

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ScyllaDBManagerClusterRegistrationManagerAPIConnectionVerifiedCondition reports whether
	// the Operator has connected to ScyllaDB Manager through verified mutual TLS.
	ScyllaDBManagerClusterRegistrationManagerAPIConnectionVerifiedCondition = "ManagerAPIConnectionVerified"
	// ScyllaDBManagerClusterRegistrationDatabaseConnectionVerifiedCondition reports whether
	// ScyllaDB Manager has verified every configured database connection property.
	ScyllaDBManagerClusterRegistrationDatabaseConnectionVerifiedCondition = "DatabaseConnectionVerified"
)

type LocalScyllaDBReference struct {
	// kind specifies the type of the resource.
	// +kubebuilder:validation:Enum=ScyllaCluster;ScyllaDBCluster;ScyllaDBDatacenter
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`
	// name specifies the name of the resource in the same namespace.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

type ScyllaDBManagerClusterRegistrationCQLAuthentication struct {
	// usernameSecretKeyRef selects the CQL username from a Secret in the registration namespace.
	// +kubebuilder:validation:Required
	UsernameSecretKeyRef corev1.SecretKeySelector `json:"usernameSecretKeyRef"`
	// passwordSecretKeyRef selects the CQL password from a Secret in the registration namespace.
	// +kubebuilder:validation:Required
	PasswordSecretKeyRef corev1.SecretKeySelector `json:"passwordSecretKeyRef"`
}

type ScyllaDBManagerClusterRegistrationAlternatorAuthentication struct {
	// accessKeyIDSecretKeyRef selects the Alternator access key ID from a Secret in the registration namespace.
	// +kubebuilder:validation:Required
	AccessKeyIDSecretKeyRef corev1.SecretKeySelector `json:"accessKeyIDSecretKeyRef"`
	// secretAccessKeySecretKeyRef selects the Alternator secret access key from a Secret in the registration namespace.
	// +kubebuilder:validation:Required
	SecretAccessKeySecretKeyRef corev1.SecretKeySelector `json:"secretAccessKeySecretKeyRef"`
}

type ScyllaDBManagerClusterRegistrationAuthentication struct {
	// cql configures username and password authentication for CQL.
	// +kubebuilder:validation:Required
	CQL *ScyllaDBManagerClusterRegistrationCQLAuthentication `json:"cql,omitempty"`
	// alternator configures access-key authentication for Alternator.
	// +optional
	Alternator *ScyllaDBManagerClusterRegistrationAlternatorAuthentication `json:"alternator,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="has(self.caConfigMapKeyRef) != has(self.caSecretKeyRef)",message="exactly one of caConfigMapKeyRef or caSecretKeyRef is required"
type ScyllaDBManagerClusterRegistrationTLSConfig struct {
	// caConfigMapKeyRef selects a PEM CA bundle from a ConfigMap in the registration namespace.
	// Mutually exclusive with caSecretKeyRef.
	// +optional
	CAConfigMapKeyRef *corev1.ConfigMapKeySelector `json:"caConfigMapKeyRef,omitempty"`
	// caSecretKeyRef selects a PEM CA bundle from a Secret in the registration namespace.
	// Mutually exclusive with caConfigMapKeyRef.
	// +optional
	CASecretKeyRef *corev1.SecretKeySelector `json:"caSecretKeyRef,omitempty"`
	// serverName is the DNS name Manager must verify against the serving certificate.
	// +kubebuilder:validation:Required
	ServerName string `json:"serverName"`
}

type ScyllaDBManagerClusterRegistrationTLS struct {
	// cql configures strict CQL server certificate verification.
	// +kubebuilder:validation:Required
	CQL *ScyllaDBManagerClusterRegistrationTLSConfig `json:"cql,omitempty"`
	// alternator configures strict Alternator server certificate verification.
	// +optional
	Alternator *ScyllaDBManagerClusterRegistrationTLSConfig `json:"alternator,omitempty"`
	// agent configures strict ScyllaDB Manager Agent server certificate verification.
	// +kubebuilder:validation:Required
	Agent *ScyllaDBManagerClusterRegistrationTLSConfig `json:"agent,omitempty"`
}

type ScyllaDBManagerClusterRegistrationManagerAPIClientCertificate struct {
	// certificateSecretKeyRef selects a PEM client certificate from a Secret in the registration namespace.
	// +kubebuilder:validation:Required
	CertificateSecretKeyRef corev1.SecretKeySelector `json:"certificateSecretKeyRef"`
	// privateKeySecretKeyRef selects the corresponding PEM private key from a Secret in the registration namespace.
	// +kubebuilder:validation:Required
	PrivateKeySecretKeyRef corev1.SecretKeySelector `json:"privateKeySecretKeyRef"`
}

// +kubebuilder:validation:XValidation:rule="has(self.caConfigMapKeyRef) != has(self.caSecretKeyRef)",message="exactly one of caConfigMapKeyRef or caSecretKeyRef is required"
type ScyllaDBManagerClusterRegistrationManagerAPI struct {
	// url is the HTTPS base URL of the ScyllaDB Manager API.
	// +kubebuilder:validation:Required
	URL string `json:"url"`
	// caConfigMapKeyRef selects the PEM CA bundle used to verify the Manager API.
	// Mutually exclusive with caSecretKeyRef.
	// +optional
	CAConfigMapKeyRef *corev1.ConfigMapKeySelector `json:"caConfigMapKeyRef,omitempty"`
	// caSecretKeyRef selects the PEM CA bundle used to verify the Manager API.
	// Mutually exclusive with caConfigMapKeyRef.
	// +optional
	CASecretKeyRef *corev1.SecretKeySelector `json:"caSecretKeyRef,omitempty"`
	// serverName is the DNS name verified against the Manager API serving certificate.
	// +kubebuilder:validation:Required
	ServerName string `json:"serverName"`
	// clientCertificate configures the client identity required by Manager mutual TLS.
	// +kubebuilder:validation:Required
	ClientCertificate ScyllaDBManagerClusterRegistrationManagerAPIClientCertificate `json:"clientCertificate"`
}

type ScyllaDBManagerClusterRegistrationSpec struct {
	// scyllaDBClusterRef specifies the typed reference to the local ScyllaDB cluster.
	// Supported kinds are ScyllaCluster, ScyllaDBCluster, and ScyllaDBDatacenter in the
	// scylla.scylladb.com API group.
	// +kubebuilder:validation:Required
	ScyllaDBClusterRef LocalScyllaDBReference `json:"scyllaDBClusterRef"`

	// managerAPI configures the verified mutual-TLS connection from Operator to Manager.
	// +kubebuilder:validation:Required
	ManagerAPI ScyllaDBManagerClusterRegistrationManagerAPI `json:"managerAPI"`

	// authentication configures database credentials. Values are read from Secrets only while
	// constructing a Manager request and are never persisted in this resource's status.
	// +kubebuilder:validation:Required
	Authentication *ScyllaDBManagerClusterRegistrationAuthentication `json:"authentication,omitempty"`

	// tls configures endpoint-specific CA trust and server-name verification.
	// +kubebuilder:validation:Required
	TLS *ScyllaDBManagerClusterRegistrationTLS `json:"tls,omitempty"`
}

type ScyllaDBManagerClusterRegistrationStatus struct {
	// observedGeneration is the most recent generation observed for this ScyllaDBManagerClusterRegistration. It corresponds to the
	// ScyllaDBManagerClusterRegistration's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// conditions hold conditions describing ScyllaDBManagerClusterRegistration state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// clusterID reflects the internal identification number of the cluster in ScyllaDB Manager state.
	// +optional
	ClusterID *string `json:"clusterID,omitempty"`

	// connectionRevision is a non-secret digest of the desired connection metadata and referenced
	// object revisions. Secret data is deliberately excluded.
	// +optional
	ConnectionRevision *string `json:"connectionRevision,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="PROGRESSING",type=string,JSONPath=".status.conditions[?(@.type=='Progressing')].status"
// +kubebuilder:printcolumn:name="DEGRADED",type=string,JSONPath=".status.conditions[?(@.type=='Degraded')].status"
// +kubebuilder:printcolumn:name="MANAGER VERIFIED",type=string,JSONPath=".status.conditions[?(@.type=='ManagerAPIConnectionVerified')].status"
// +kubebuilder:printcolumn:name="DB VERIFIED",type=string,JSONPath=".status.conditions[?(@.type=='DatabaseConnectionVerified')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

type ScyllaDBManagerClusterRegistration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines the desired state of ScyllaDBManagerClusterRegistration.
	Spec ScyllaDBManagerClusterRegistrationSpec `json:"spec,omitempty"`

	// status reflects the observed state of ScyllaDBManagerClusterRegistration.
	Status ScyllaDBManagerClusterRegistrationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ScyllaDBManagerClusterRegistrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScyllaDBManagerClusterRegistration `json:"items"`
}
