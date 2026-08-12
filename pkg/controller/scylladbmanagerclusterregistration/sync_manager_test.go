// Copyright (C) 2025 ScyllaDB

package scylladbmanagerclusterregistration

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/managerclientsecure"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

func Test_scyllaDBManagerClusterName(t *testing.T) {
	tt := []struct {
		name     string
		smcr     *scyllav1alpha1.ScyllaDBManagerClusterRegistration
		expected string
	}{
		{
			name: "basic",
			smcr: &scyllav1alpha1.ScyllaDBManagerClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "basic",
					Namespace: "scylla",
				},
				Spec: scyllav1alpha1.ScyllaDBManagerClusterRegistrationSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
				},
			},
			expected: "ScyllaDBDatacenter/basic",
		},
		{
			name: "name-override annotation, no global manager label",
			smcr: &scyllav1alpha1.ScyllaDBManagerClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "basic",
					Namespace: "scylla",
					Annotations: map[string]string{
						"internal.scylla-operator.scylladb.com/scylladb-manager-cluster-name-override": "scylla/scylla",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerClusterRegistrationSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
				},
			},
			expected: "scylla/scylla",
		},
		{
			name: "no name-override annotation, global manager label",
			smcr: &scyllav1alpha1.ScyllaDBManagerClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "basic",
					Namespace: "scylla",
					Labels: map[string]string{
						"internal.scylla-operator.scylladb.com/global-scylladb-manager": "true",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerClusterRegistrationSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
				},
			},
			expected: "scylla/ScyllaDBDatacenter/basic",
		},
		{
			name: "name-override annotation, global manager label",
			smcr: &scyllav1alpha1.ScyllaDBManagerClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "basic",
					Namespace: "scylla",
					Annotations: map[string]string{
						"internal.scylla-operator.scylladb.com/scylladb-manager-cluster-name-override": "scylla/scylla",
					},
					Labels: map[string]string{
						"internal.scylla-operator.scylladb.com/global-scylladb-manager": "true",
					},
				},
				Spec: scyllav1alpha1.ScyllaDBManagerClusterRegistrationSpec{
					ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
						Name: "basic",
						Kind: "ScyllaDBDatacenter",
					},
				},
			},
			expected: "scylla/scylla",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := scyllaDBManagerClusterName(tc.smcr)
			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("expected and got ScyllaDB Manager clusters differ:\n%s\n", cmp.Diff(tc.expected, got))
			}
		})
	}
}

func Test_makeRequiredScyllaDBManagerCluster(t *testing.T) {
	t.Parallel()

	smcr := &scyllav1alpha1.ScyllaDBManagerClusterRegistration{
		Spec: scyllav1alpha1.ScyllaDBManagerClusterRegistrationSpec{
			TLS: &scyllav1alpha1.ScyllaDBManagerClusterRegistrationTLS{
				CQL:        &scyllav1alpha1.ScyllaDBManagerClusterRegistrationTLSConfig{ServerName: "db.svc"},
				Alternator: &scyllav1alpha1.ScyllaDBManagerClusterRegistrationTLSConfig{ServerName: "alternator.svc"},
				Agent:      &scyllav1alpha1.ScyllaDBManagerClusterRegistrationTLSConfig{ServerName: "db.svc"},
			},
		},
	}
	connection := &resolvedScyllaDBManagerClusterConnection{
		AuthToken:                 "agent-token",
		Username:                  "cql-user",
		Password:                  "cql-password",
		AlternatorAccessKeyID:     "alternator-key",
		AlternatorSecretAccessKey: "alternator-secret",
		CQLCA:                     []byte("cql-ca"),
		CQLClientCertificate:      []byte("cql-client-cert"),
		CQLClientPrivateKey:       []byte("cql-client-key"),
		AlternatorCA:              []byte("alternator-ca"),
		AgentCA:                   []byte("agent-ca"),
		Revision:                  "non-secret-revision",
	}
	expected := &managerclientsecure.Cluster{
		Name:                      "basic",
		Host:                      "basic-client.sophena.svc",
		AuthToken:                 "agent-token",
		Username:                  "cql-user",
		Password:                  "cql-password",
		AlternatorAccessKeyID:     "alternator-key",
		AlternatorSecretAccessKey: "alternator-secret",
		CQLCAFile:                 []byte("cql-ca"),
		CQLServerName:             "db.svc",
		SSLUserCertFile:           []byte("cql-client-cert"),
		SSLUserKeyFile:            []byte("cql-client-key"),
		AlternatorCAFile:          []byte("alternator-ca"),
		AlternatorServerName:      "alternator.svc",
		AgentCAFile:               []byte("agent-ca"),
		AgentServerName:           "db.svc",
		Labels: map[string]string{
			"scylla-operator.scylladb.com/managed-hash": "non-secret-revision",
			"scylla-operator.scylladb.com/owner-uid":    "uid",
		},
		WithoutRepair: true,
	}

	got := makeRequiredScyllaDBManagerCluster("basic", "uid", "basic-client.sophena.svc", smcr, connection)
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("expected and got ScyllaDB Manager clusters differ:\n%s\n", cmp.Diff(expected, got))
	}
}

func TestRegistrationConnectionTargetScyllaCluster(t *testing.T) {
	t.Parallel()

	availableMembers := int32(3)
	sc := &scyllav1.ScyllaCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "scylladb", Namespace: "sophena"},
		Status:     scyllav1.ScyllaClusterStatus{AvailableMembers: &availableMembers},
	}
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	if err := indexer.Add(sc); err != nil {
		t.Fatalf("can't add ScyllaCluster to indexer: %v", err)
	}
	controller := &Controller{
		scyllaClusterLister: scyllav1listers.NewScyllaClusterLister(indexer),
	}
	smcr := &scyllav1alpha1.ScyllaDBManagerClusterRegistration{
		ObjectMeta: metav1.ObjectMeta{Namespace: "sophena", Generation: 1},
		Spec: scyllav1alpha1.ScyllaDBManagerClusterRegistrationSpec{
			ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
				Kind: scyllav1.ScyllaClusterGVK.Kind,
				Name: sc.Name,
			},
		},
	}

	host, agentTokenSecretName, progressingCondition, err := controller.registrationConnectionTarget(smcr)
	if err != nil {
		t.Fatalf("can't resolve direct stable ScyllaCluster target: %v", err)
	}
	if progressingCondition != nil {
		t.Fatalf("unexpected progressing condition: %#v", progressingCondition)
	}
	if host != "scylladb-client.sophena.svc" {
		t.Fatalf("expected stable shared Service DNS, got %q", host)
	}
	if agentTokenSecretName != "scylladb-auth-token" {
		t.Fatalf("expected stable Agent token Secret, got %q", agentTokenSecretName)
	}
}

func TestResolveRegistrationConnectionRotationAndNonSecretRevision(t *testing.T) {
	t.Parallel()

	newController := func(password, passwordResourceVersion, databaseClientResourceVersion string) *Controller {
		secretIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
		configMapIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
		for _, secret := range []*corev1.Secret{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "scylladb-auth-token", Namespace: "sophena", ResourceVersion: "agent-rv"},
				Data:       map[string][]byte{"auth-token.yaml": []byte("auth_token: agent-token")},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "manager-client", Namespace: "sophena", ResourceVersion: "manager-rv"},
				Data: map[string][]byte{
					"ca.crt":  []byte("manager-ca"),
					"tls.crt": []byte("client-cert"),
					"tls.key": []byte("client-key"),
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "database-auth", Namespace: "sophena", ResourceVersion: passwordResourceVersion},
				Data: map[string][]byte{
					"username": []byte("cql-user"),
					"password": []byte(password),
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "database-client-tls", Namespace: "sophena", ResourceVersion: databaseClientResourceVersion},
				Data: map[string][]byte{
					corev1.TLSCertKey:       []byte("database-client-cert"),
					corev1.TLSPrivateKeyKey: []byte("database-client-key"),
				},
			},
		} {
			if err := secretIndexer.Add(secret); err != nil {
				t.Fatalf("can't add Secret to indexer: %v", err)
			}
		}
		if err := configMapIndexer.Add(&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "database-ca", Namespace: "sophena", ResourceVersion: "database-ca-rv"},
			Data:       map[string]string{"ca-bundle.crt": "database-ca"},
		}); err != nil {
			t.Fatalf("can't add ConfigMap to indexer: %v", err)
		}
		return &Controller{
			secretLister:    corev1listers.NewSecretLister(secretIndexer),
			configMapLister: corev1listers.NewConfigMapLister(configMapIndexer),
		}
	}

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
			Authentication: &scyllav1alpha1.ScyllaDBManagerClusterRegistrationAuthentication{
				CQL: &scyllav1alpha1.ScyllaDBManagerClusterRegistrationCQLAuthentication{
					UsernameSecretKeyRef: corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "database-auth"}, Key: "username"},
					PasswordSecretKeyRef: corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "database-auth"}, Key: "password"},
				},
			},
			TLS: &scyllav1alpha1.ScyllaDBManagerClusterRegistrationTLS{
				CQL: &scyllav1alpha1.ScyllaDBManagerClusterRegistrationTLSConfig{
					CAConfigMapKeyRef: &corev1.ConfigMapKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "database-ca"}, Key: "ca-bundle.crt"},
					ServerName:        "scylladb-client.sophena.svc",
					ClientCertificate: &scyllav1alpha1.ScyllaDBManagerClusterRegistrationTLSClientCertificate{
						CertificateSecretKeyRef: corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "database-client-tls"}, Key: corev1.TLSCertKey},
						PrivateKeySecretKeyRef:  corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "database-client-tls"}, Key: corev1.TLSPrivateKeyKey},
					},
				},
				Agent: &scyllav1alpha1.ScyllaDBManagerClusterRegistrationTLSConfig{
					CAConfigMapKeyRef: &corev1.ConfigMapKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "database-ca"}, Key: "ca-bundle.crt"},
					ServerName:        "scylladb-client.sophena.svc",
				},
			},
		},
	}

	first, err := newController("first-password", "auth-rv-1", "database-client-rv-1").resolveRegistrationConnection(smcr, "scylladb-client.sophena.svc", "scylladb-auth-token")
	if err != nil {
		t.Fatalf("can't resolve registration: %v", err)
	}
	if first.Password != "first-password" || string(first.CQLCA) != "database-ca" || string(first.CQLClientCertificate) != "database-client-cert" || string(first.CQLClientPrivateKey) != "database-client-key" || string(first.AgentCA) != "database-ca" {
		t.Fatalf("unexpected resolved connection: %#v", first)
	}
	if strings.Contains(first.Revision, "first-password") || strings.Contains(first.Revision, "agent-token") {
		t.Fatal("non-secret revision contains secret material")
	}

	sameRevision, err := newController("different-bytes", "auth-rv-1", "database-client-rv-1").resolveRegistrationConnection(smcr, "scylladb-client.sophena.svc", "scylladb-auth-token")
	if err != nil {
		t.Fatalf("can't resolve registration with changed bytes: %v", err)
	}
	if first.Revision != sameRevision.Revision {
		t.Fatal("revision must not hash secret bytes")
	}

	rotated, err := newController("different-bytes", "auth-rv-2", "database-client-rv-1").resolveRegistrationConnection(smcr, "scylladb-client.sophena.svc", "scylladb-auth-token")
	if err != nil {
		t.Fatalf("can't resolve registration after rotation: %v", err)
	}
	if first.Revision == rotated.Revision {
		t.Fatal("Secret resourceVersion rotation must change the connection revision")
	}

	rotatedClientCertificate, err := newController("first-password", "auth-rv-1", "database-client-rv-2").resolveRegistrationConnection(smcr, "scylladb-client.sophena.svc", "scylladb-auth-token")
	if err != nil {
		t.Fatalf("can't resolve registration after CQL client certificate rotation: %v", err)
	}
	if first.Revision == rotatedClientCertificate.Revision {
		t.Fatal("CQL client certificate Secret resourceVersion rotation must change the connection revision")
	}
}

func TestDatabaseConnectionVerifiedRequiresAgentTLS(t *testing.T) {
	t.Parallel()

	smcr := &scyllav1alpha1.ScyllaDBManagerClusterRegistration{ObjectMeta: metav1.ObjectMeta{Generation: 4}}
	status := &scyllav1alpha1.ScyllaDBManagerClusterRegistrationStatus{}
	setDatabaseConnectionVerificationFromManager(status, smcr, []*managerclientsecure.ClusterStatusItem{
		{CQLTLSVerified: true, CQLAuthVerified: true, AgentTLSVerified: false},
	})
	condition := apimeta.FindStatusCondition(status.Conditions, scyllav1alpha1.ScyllaDBManagerClusterRegistrationDatabaseConnectionVerifiedCondition)
	if condition == nil || condition.Status != metav1.ConditionFalse || condition.ObservedGeneration != smcr.Generation {
		t.Fatalf("expected current DatabaseConnectionVerified=False when Agent TLS is unverified, got %#v", condition)
	}

	setDatabaseConnectionVerificationFromManager(status, smcr, []*managerclientsecure.ClusterStatusItem{
		{CQLTLSVerified: true, CQLAuthVerified: true, AgentTLSVerified: true},
	})
	condition = apimeta.FindStatusCondition(status.Conditions, scyllav1alpha1.ScyllaDBManagerClusterRegistrationDatabaseConnectionVerifiedCondition)
	if condition == nil || condition.Status != metav1.ConditionTrue || condition.ObservedGeneration != smcr.Generation {
		t.Fatalf("expected current DatabaseConnectionVerified=True after full verification, got %#v", condition)
	}
}

func TestHealthyVerificationPollPreservesConditionTransitionTimes(t *testing.T) {
	t.Parallel()

	transitionTime := metav1.NewTime(time.Unix(123, 0))
	smcr := &scyllav1alpha1.ScyllaDBManagerClusterRegistration{ObjectMeta: metav1.ObjectMeta{Generation: 7}}
	status := &scyllav1alpha1.ScyllaDBManagerClusterRegistrationStatus{
		Conditions: []metav1.Condition{
			{
				Type:               scyllav1alpha1.ScyllaDBManagerClusterRegistrationManagerAPIConnectionVerifiedCondition,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: smcr.Generation,
				Reason:             "VerifiedManagerAPIConnection",
				LastTransitionTime: transitionTime,
			},
			{
				Type:               scyllav1alpha1.ScyllaDBManagerClusterRegistrationDatabaseConnectionVerifiedCondition,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: smcr.Generation,
				Reason:             "VerifiedDatabaseConnection",
				LastTransitionTime: transitionTime,
			},
		},
	}

	setManagerAPIConnectionVerified(status, smcr, metav1.ConditionTrue, "VerifiedManagerAPIConnection", "Connected to ScyllaDB Manager using verified mutual TLS.")
	setDatabaseConnectionVerificationFromManager(status, smcr, []*managerclientsecure.ClusterStatusItem{
		{CQLTLSVerified: true, CQLAuthVerified: true, AgentTLSVerified: true},
	})

	for _, conditionType := range []string{
		scyllav1alpha1.ScyllaDBManagerClusterRegistrationManagerAPIConnectionVerifiedCondition,
		scyllav1alpha1.ScyllaDBManagerClusterRegistrationDatabaseConnectionVerifiedCondition,
	} {
		condition := apimeta.FindStatusCondition(status.Conditions, conditionType)
		if condition == nil || !condition.LastTransitionTime.Equal(&transitionTime) {
			t.Fatalf("healthy poll reset %s transition time: %#v", conditionType, condition)
		}
	}
}
