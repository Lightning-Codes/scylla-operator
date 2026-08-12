// Copyright (C) 2025 ScyllaDB

package validation

import (
	"os"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"
)

func TestValidateScyllaDBManagerClusterRegistration(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name                               string
		scyllaDBManagerClusterRegistration *scyllav1alpha1.ScyllaDBManagerClusterRegistration
		expectedErrorList                  field.ErrorList
		expectedErrorString                string
	}{
		{
			name:                               "valid",
			scyllaDBManagerClusterRegistration: newValidScyllaDBManagerClusterRegistration(),
			expectedErrorList:                  nil,
			expectedErrorString:                ``,
		},
		{
			name: "user-owned registration without global manager label",
			scyllaDBManagerClusterRegistration: func() *scyllav1alpha1.ScyllaDBManagerClusterRegistration {
				smcr := newValidScyllaDBManagerClusterRegistration()

				smcr.Labels = map[string]string{}

				return smcr
			}(),
			expectedErrorList:   nil,
			expectedErrorString: ``,
		},
		{
			name: "global manager label is not required for validation",
			scyllaDBManagerClusterRegistration: func() *scyllav1alpha1.ScyllaDBManagerClusterRegistration {
				smcr := newValidScyllaDBManagerClusterRegistration()

				smcr.Labels = map[string]string{
					"internal.scylla-operator.scylladb.com/global-scylladb-manager": "invalid",
				}

				return smcr
			}(),
			expectedErrorList:   nil,
			expectedErrorString: ``,
		},
		{
			name: "empty scyllaDBClusterRef name",
			scyllaDBManagerClusterRegistration: func() *scyllav1alpha1.ScyllaDBManagerClusterRegistration {
				smcr := newValidScyllaDBManagerClusterRegistration()

				smcr.Spec.ScyllaDBClusterRef = scyllav1alpha1.LocalScyllaDBReference{
					Name: "",
					Kind: "ScyllaDBDatacenter",
				}

				return smcr
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.scyllaDBClusterRef.name",
					BadValue: ``,
					Detail:   ``,
				},
			},
			expectedErrorString: `spec.scyllaDBClusterRef.name: Required value`,
		},
		{
			name: "invalid scyllaDBClusterRef name",
			scyllaDBManagerClusterRegistration: func() *scyllav1alpha1.ScyllaDBManagerClusterRegistration {
				smcr := newValidScyllaDBManagerClusterRegistration()

				smcr.Spec.ScyllaDBClusterRef = scyllav1alpha1.LocalScyllaDBReference{
					Name: "-invalid",
					Kind: "ScyllaDBDatacenter",
				}

				return smcr
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.scyllaDBClusterRef.name",
					BadValue: `-invalid`,
					Detail:   `a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
				},
			},
			expectedErrorString: `spec.scyllaDBClusterRef.name: Invalid value: "-invalid": a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`,
		},
		{
			name: "empty scyllaDBClusterRef kind",
			scyllaDBManagerClusterRegistration: func() *scyllav1alpha1.ScyllaDBManagerClusterRegistration {
				smcr := newValidScyllaDBManagerClusterRegistration()

				smcr.Spec.ScyllaDBClusterRef = scyllav1alpha1.LocalScyllaDBReference{
					Name: "basic",
					Kind: "",
				}

				return smcr
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.scyllaDBClusterRef.kind",
					BadValue: ``,
					Detail:   ``,
				},
			},
			expectedErrorString: `spec.scyllaDBClusterRef.kind: Required value`,
		},
		{
			name: "stable ScyllaCluster reference",
			scyllaDBManagerClusterRegistration: func() *scyllav1alpha1.ScyllaDBManagerClusterRegistration {
				smcr := newValidScyllaDBManagerClusterRegistration()

				smcr.Spec.ScyllaDBClusterRef = scyllav1alpha1.LocalScyllaDBReference{
					Name: "basic",
					Kind: "ScyllaCluster",
				}

				return smcr
			}(),
			expectedErrorList:   nil,
			expectedErrorString: ``,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			errList := ValidateScyllaDBManagerClusterRegistration(tc.scyllaDBManagerClusterRegistration)
			if !reflect.DeepEqual(errList, tc.expectedErrorList) {
				t.Errorf("expected and actual error lists differ: %s", cmp.Diff(tc.expectedErrorList, errList))
			}

			var errStr string
			if agg := errList.ToAggregate(); agg != nil {
				errStr = agg.Error()
			}
			if !reflect.DeepEqual(errStr, tc.expectedErrorString) {
				t.Errorf("expected and actual error strings differ: %s", cmp.Diff(tc.expectedErrorString, errStr))
			}
		})
	}
}

func TestValidateScyllaDBManagerClusterRegistrationUpdate(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name                string
		old                 *scyllav1alpha1.ScyllaDBManagerClusterRegistration
		new                 *scyllav1alpha1.ScyllaDBManagerClusterRegistration
		expectedErrorList   field.ErrorList
		expectedErrorString string
	}{
		{
			name:                "identity",
			old:                 newValidScyllaDBManagerClusterRegistration(),
			new:                 newValidScyllaDBManagerClusterRegistration(),
			expectedErrorList:   nil,
			expectedErrorString: ``,
		},
		{
			name: "scyllaDBClusterRef name changed",
			old: func() *scyllav1alpha1.ScyllaDBManagerClusterRegistration {
				smcr := newValidScyllaDBManagerClusterRegistration()

				smcr.Spec.ScyllaDBClusterRef = scyllav1alpha1.LocalScyllaDBReference{
					Name: "basic",
					Kind: "ScyllaDBDatacenter",
				}

				return smcr
			}(),
			new: func() *scyllav1alpha1.ScyllaDBManagerClusterRegistration {
				smcr := newValidScyllaDBManagerClusterRegistration()

				smcr.Spec.ScyllaDBClusterRef = scyllav1alpha1.LocalScyllaDBReference{
					Name: "new-basic",
					Kind: "ScyllaDBDatacenter",
				}

				return smcr
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.scyllaDBClusterRef.name",
					BadValue: "new-basic",
					Detail:   "field is immutable",
				},
			},
			expectedErrorString: `spec.scyllaDBClusterRef.name: Invalid value: "new-basic": field is immutable`,
		},
		{
			name: "scyllaDBClusterRef kind changed",
			old: func() *scyllav1alpha1.ScyllaDBManagerClusterRegistration {
				smcr := newValidScyllaDBManagerClusterRegistration()

				smcr.Spec.ScyllaDBClusterRef = scyllav1alpha1.LocalScyllaDBReference{
					Name: "basic",
					Kind: "ScyllaDBDatacenter",
				}

				return smcr
			}(),
			new: func() *scyllav1alpha1.ScyllaDBManagerClusterRegistration {
				smcr := newValidScyllaDBManagerClusterRegistration()

				smcr.Spec.ScyllaDBClusterRef = scyllav1alpha1.LocalScyllaDBReference{
					Name: "basic",
					Kind: "ScyllaDBCluster",
				}

				return smcr
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.scyllaDBClusterRef.kind",
					BadValue: "ScyllaDBCluster",
					Detail:   "field is immutable",
				},
			},
			expectedErrorString: `spec.scyllaDBClusterRef.kind: Invalid value: "ScyllaDBCluster": field is immutable`,
		},
		{
			name: "internal.scylla-operator.scylladb.com/scylladb-manager-cluster-name-override annotation changed",
			old:  newValidScyllaDBManagerClusterRegistration(),
			new: func() *scyllav1alpha1.ScyllaDBManagerClusterRegistration {
				smcr := newValidScyllaDBManagerClusterRegistration()

				if smcr.Annotations == nil {
					smcr.Annotations = map[string]string{}
				}
				smcr.Annotations[naming.ScyllaDBManagerClusterRegistrationNameOverrideAnnotation] = "scylla/scylla"

				return smcr
			}(),
			expectedErrorList: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "metadata.annotations[internal.scylla-operator.scylladb.com/scylladb-manager-cluster-name-override]",
					BadValue: "scylla/scylla",
					Detail:   "field is immutable",
				},
			},
			expectedErrorString: `metadata.annotations[internal.scylla-operator.scylladb.com/scylladb-manager-cluster-name-override]: Invalid value: "scylla/scylla": field is immutable`,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			errList := ValidateScyllaDBManagerClusterRegistrationUpdate(tc.new, tc.old)
			if !reflect.DeepEqual(errList, tc.expectedErrorList) {
				t.Errorf("expected and actual error lists differ: %s", cmp.Diff(tc.expectedErrorList, errList))
			}

			errStr := ""
			if agg := errList.ToAggregate(); agg != nil {
				errStr = agg.Error()
			}
			if !reflect.DeepEqual(errStr, tc.expectedErrorString) {
				t.Errorf("expected and actual error strings differ: %s", cmp.Diff(tc.expectedErrorString, errStr))
			}
		})
	}
}

func TestValidateScyllaDBManagerClusterRegistrationCASelectors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		mutate        func(*scyllav1alpha1.ScyllaDBManagerClusterRegistration)
		expectedField string
	}{
		{
			name: "manager API CA missing",
			mutate: func(smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration) {
				smcr.Spec.ManagerAPI.CASecretKeyRef = nil
			},
			expectedField: "spec.managerAPI",
		},
		{
			name: "manager API CA selectors are mutually exclusive",
			mutate: func(smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration) {
				smcr.Spec.ManagerAPI.CAConfigMapKeyRef = &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "manager-ca"},
					Key:                  "ca-bundle.crt",
				}
			},
			expectedField: "spec.managerAPI",
		},
		{
			name: "CQL CA missing",
			mutate: func(smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration) {
				smcr.Spec.TLS.CQL.CAConfigMapKeyRef = nil
			},
			expectedField: "spec.tls.cql",
		},
		{
			name: "CQL CA selectors are mutually exclusive",
			mutate: func(smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration) {
				smcr.Spec.TLS.CQL.CASecretKeyRef = &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "database-ca"},
					Key:                  "ca.crt",
				}
			},
			expectedField: "spec.tls.cql",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			smcr := newValidScyllaDBManagerClusterRegistration()
			tc.mutate(smcr)
			errs := ValidateScyllaDBManagerClusterRegistration(smcr)
			if len(errs) != 1 {
				t.Fatalf("expected one validation error, got %v", errs)
			}
			if errs[0].Field != tc.expectedField {
				t.Fatalf("expected error field %q, got %q", tc.expectedField, errs[0].Field)
			}
		})
	}
}

func newValidScyllaDBManagerClusterRegistration() *scyllav1alpha1.ScyllaDBManagerClusterRegistration {
	return &scyllav1alpha1.ScyllaDBManagerClusterRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "basic",
			UID:  "uid",
			Labels: map[string]string{
				"internal.scylla-operator.scylladb.com/global-scylladb-manager": "true",
			},
		},
		Spec: scyllav1alpha1.ScyllaDBManagerClusterRegistrationSpec{
			ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
				Name: "basic",
				Kind: "ScyllaDBDatacenter",
			},
			ManagerAPI: scyllav1alpha1.ScyllaDBManagerClusterRegistrationManagerAPI{
				URL: "https://scylla-manager.scylla-manager.svc:5080/api/v1",
				CASecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "manager-client-tls"},
					Key:                  "ca.crt",
				},
				ServerName: "scylla-manager.scylla-manager.svc",
				ClientCertificate: scyllav1alpha1.ScyllaDBManagerClusterRegistrationManagerAPIClientCertificate{
					CertificateSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "manager-client-tls"},
						Key:                  corev1.TLSCertKey,
					},
					PrivateKeySecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "manager-client-tls"},
						Key:                  corev1.TLSPrivateKeyKey,
					},
				},
			},
			Authentication: &scyllav1alpha1.ScyllaDBManagerClusterRegistrationAuthentication{
				CQL: &scyllav1alpha1.ScyllaDBManagerClusterRegistrationCQLAuthentication{
					UsernameSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "database-auth"},
						Key:                  "username",
					},
					PasswordSecretKeyRef: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "database-auth"},
						Key:                  "password",
					},
				},
			},
			TLS: &scyllav1alpha1.ScyllaDBManagerClusterRegistrationTLS{
				CQL: &scyllav1alpha1.ScyllaDBManagerClusterRegistrationTLSConfig{
					CAConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "database-ca"},
						Key:                  "ca-bundle.crt",
					},
					ServerName: "basic-client.default.svc",
				},
				Agent: &scyllav1alpha1.ScyllaDBManagerClusterRegistrationTLSConfig{
					CAConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "database-ca"},
						Key:                  "ca-bundle.crt",
					},
					ServerName: "basic-client.default.svc",
				},
			},
		},
	}
}

func TestScyllaDBManagerClusterRegistrationCRDAdmitsStableScyllaClusterKind(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("../v1alpha1/scylla.scylladb.com_scylladbmanagerclusterregistrations.yaml")
	if err != nil {
		t.Fatalf("can't read generated registration CRD: %v", err)
	}
	crd := &apiextensionsv1.CustomResourceDefinition{}
	if err := yaml.UnmarshalStrict(data, crd); err != nil {
		t.Fatalf("can't decode generated registration CRD: %v", err)
	}
	if len(crd.Spec.Versions) != 1 || crd.Spec.Versions[0].Schema == nil || crd.Spec.Versions[0].Schema.OpenAPIV3Schema == nil {
		t.Fatalf("registration CRD is missing its version schema: %#v", crd.Spec.Versions)
	}

	specSchema := crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"]
	kindSchema := specSchema.Properties["scyllaDBClusterRef"].Properties["kind"]
	want := []apiextensionsv1.JSON{
		{Raw: []byte(`"ScyllaCluster"`)},
		{Raw: []byte(`"ScyllaDBCluster"`)},
		{Raw: []byte(`"ScyllaDBDatacenter"`)},
	}
	if !reflect.DeepEqual(kindSchema.Enum, want) {
		t.Fatalf("generated LocalScyllaDBReference kind enum does not admit direct %s: expected %#v, got %#v", scyllav1.ScyllaClusterGVK.Kind, want, kindSchema.Enum)
	}
	if wantRequired := []string{"authentication", "managerAPI", "scyllaDBClusterRef", "tls"}; !reflect.DeepEqual(specSchema.Required, wantRequired) {
		t.Fatalf("generated registration spec does not require the secure connection contract: expected %v, got %v", wantRequired, specSchema.Required)
	}
	const exactlyOneCARule = "has(self.caConfigMapKeyRef) != has(self.caSecretKeyRef)"
	managerAPISchema := specSchema.Properties["managerAPI"]
	if len(managerAPISchema.XValidations) != 1 || managerAPISchema.XValidations[0].Rule != exactlyOneCARule {
		t.Fatalf("generated Manager API schema does not enforce exactly one CA selector: %#v", managerAPISchema.XValidations)
	}
	tlsSchema := specSchema.Properties["tls"]
	for _, endpoint := range []string{"cql", "alternator", "agent"} {
		endpointSchema := tlsSchema.Properties[endpoint]
		if len(endpointSchema.XValidations) != 1 || endpointSchema.XValidations[0].Rule != exactlyOneCARule {
			t.Fatalf("generated %s TLS schema does not enforce exactly one CA selector: %#v", endpoint, endpointSchema.XValidations)
		}
	}
}
