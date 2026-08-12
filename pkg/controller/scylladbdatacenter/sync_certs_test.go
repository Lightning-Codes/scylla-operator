package scylladbdatacenter

import (
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav1alpha1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/kubecrypto"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func Test_makeScyllaConnectionConfig(t *testing.T) {
	tt := []struct {
		name            string
		sdc             *scyllav1alpha1.ScyllaDBDatacenter
		secrets         map[string]*corev1.Secret
		configMaps      map[string]*corev1.ConfigMap
		cqlsIngressPort int
		expected        *corev1.Secret
		expectedError   error
	}{
		{
			name: "single domain with port will generate bundle using explicit port",
			sdc: &scyllav1alpha1.ScyllaDBDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "bar",
				},
				Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
					ClusterName: "bar",
					DNSDomains: []string{
						"my-domain",
					},
					DatacenterName: pointer.Ptr("us-east-1"),
				},
			},
			secrets: map[string]*corev1.Secret{
				"bar-local-user-admin": {
					Data: map[string][]byte{
						"tls.crt": []byte("admin-certificate-data"),
						"tls.key": []byte("admin-certificate-key"),
					},
				},
			},
			configMaps: map[string]*corev1.ConfigMap{
				"bar-local-serving-ca": {
					Data: map[string]string{
						"ca-bundle.crt": "serving-certificate-data",
					},
				},
			},
			cqlsIngressPort: 9142,
			expected: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "bar-local-cql-connection-configs-admin",
					Labels: map[string]string{
						"app":                          "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"app.kubernetes.io/name":       "scylla",
						"scylla/cluster":               "bar",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBDatacenter",
							Name:               "bar",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				Type: "Opaque",
				Data: map[string][]byte{
					"my-domain": []byte(strings.TrimPrefix(`
apiVersion: cqlclient.scylla.scylladb.com/v1alpha1
authInfos:
  admin:
    clientCertificateData: YWRtaW4tY2VydGlmaWNhdGUtZGF0YQ==
    clientKeyData: YWRtaW4tY2VydGlmaWNhdGUta2V5
    password: cassandra
    username: cassandra
contexts:
  default:
    authInfoName: admin
    datacenterName: us-east-1
currentContext: default
datacenters:
  us-east-1:
    certificateAuthorityData: c2VydmluZy1jZXJ0aWZpY2F0ZS1kYXRh
    nodeDomain: cql.my-domain
    server: cql.my-domain:9142
kind: CQLConnectionConfig
parameters:
  defaultConsistency: QUORUM
  defaultSerialConsistency: SERIAL
`, "\n")),
				},
			},
			expectedError: nil,
		},
		{
			name: "multi domain will generate multiple bundles",
			sdc: &scyllav1alpha1.ScyllaDBDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "bar",
				},
				Spec: scyllav1alpha1.ScyllaDBDatacenterSpec{
					ClusterName: "bar",
					DNSDomains: []string{
						"my-domain",
						"my-private-domain",
					},
					DatacenterName: pointer.Ptr("us-east-1"),
				},
			},
			secrets: map[string]*corev1.Secret{
				"bar-local-user-admin": {
					Data: map[string][]byte{
						"tls.crt": []byte("admin-certificate-data"),
						"tls.key": []byte("admin-certificate-key"),
					},
				},
			},
			configMaps: map[string]*corev1.ConfigMap{
				"bar-local-serving-ca": {
					Data: map[string]string{
						"ca-bundle.crt": "serving-certificate-data",
					},
				},
			},
			cqlsIngressPort: 0,
			expected: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo-ns",
					Name:      "bar-local-cql-connection-configs-admin",
					Labels: map[string]string{
						"app":                          "scylla",
						"app.kubernetes.io/managed-by": "scylla-operator",
						"app.kubernetes.io/name":       "scylla",
						"scylla/cluster":               "bar",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "scylla.scylladb.com/v1alpha1",
							Kind:               "ScyllaDBDatacenter",
							Name:               "bar",
							Controller:         pointer.Ptr(true),
							BlockOwnerDeletion: pointer.Ptr(true),
						},
					},
				},
				Type: "Opaque",
				Data: map[string][]byte{
					"my-domain": []byte(strings.TrimPrefix(`
apiVersion: cqlclient.scylla.scylladb.com/v1alpha1
authInfos:
  admin:
    clientCertificateData: YWRtaW4tY2VydGlmaWNhdGUtZGF0YQ==
    clientKeyData: YWRtaW4tY2VydGlmaWNhdGUta2V5
    password: cassandra
    username: cassandra
contexts:
  default:
    authInfoName: admin
    datacenterName: us-east-1
currentContext: default
datacenters:
  us-east-1:
    certificateAuthorityData: c2VydmluZy1jZXJ0aWZpY2F0ZS1kYXRh
    nodeDomain: cql.my-domain
    server: cql.my-domain
kind: CQLConnectionConfig
parameters:
  defaultConsistency: QUORUM
  defaultSerialConsistency: SERIAL
`, "\n")),
					"my-private-domain": []byte(strings.TrimPrefix(`
apiVersion: cqlclient.scylla.scylladb.com/v1alpha1
authInfos:
  admin:
    clientCertificateData: YWRtaW4tY2VydGlmaWNhdGUtZGF0YQ==
    clientKeyData: YWRtaW4tY2VydGlmaWNhdGUta2V5
    password: cassandra
    username: cassandra
contexts:
  default:
    authInfoName: admin
    datacenterName: us-east-1
currentContext: default
datacenters:
  us-east-1:
    certificateAuthorityData: c2VydmluZy1jZXJ0aWZpY2F0ZS1kYXRh
    nodeDomain: cql.my-private-domain
    server: cql.my-private-domain
kind: CQLConnectionConfig
parameters:
  defaultConsistency: QUORUM
  defaultSerialConsistency: SERIAL
`, "\n")),
				},
			},
			expectedError: nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got, err := makeScyllaConnectionConfig(tc.sdc, tc.secrets, tc.configMaps, tc.cqlsIngressPort)
			if !reflect.DeepEqual(err, tc.expectedError) {
				t.Errorf("expected error %#v, got %#v", tc.expectedError, err)
			}

			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("expected and actual connection configs differ: %s", cmp.Diff(tc.expected, got))
			}
		})
	}
}

func TestScyllaServingDNSNamesIncludeStableSharedClientService(t *testing.T) {
	t.Parallel()

	if got := naming.GetScyllaClusterLocalServingCAName("scylladb"); got != "scylladb-local-serving-ca" {
		t.Fatalf("unexpected shared serving CA ConfigMap name: %q", got)
	}
	if got := naming.GetScyllaClusterLocalServingCertName("scylladb"); got != "scylladb-local-serving-certs" {
		t.Fatalf("unexpected shared serving certificate Secret name: %q", got)
	}
	if kubecrypto.CABundleKey != "ca-bundle.crt" {
		t.Fatalf("unexpected shared serving CA ConfigMap key: %q", kubecrypto.CABundleKey)
	}

	serviceMap := map[string]*corev1.Service{
		"scylladb-client": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "scylladb-client",
				Namespace: "sophena",
				Labels: map[string]string{
					naming.ScyllaServiceTypeLabel: string(naming.ScyllaServiceTypeIdentity),
				},
			},
			Spec: corev1.ServiceSpec{ClusterIP: "10.96.0.10"},
		},
		"scylladb-dc-ad-1-0": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "scylladb-dc-ad-1-0",
				Namespace: "sophena",
				Labels: map[string]string{
					naming.ScyllaServiceTypeLabel: string(naming.ScyllaServiceTypeMember),
				},
			},
			Spec: corev1.ServiceSpec{ClusterIP: "10.96.0.11"},
		},
		"scylladb-dc-ad-2-0": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "scylladb-dc-ad-2-0",
				Namespace: "sophena",
				Labels: map[string]string{
					naming.ScyllaServiceTypeLabel: string(naming.ScyllaServiceTypeMember),
				},
			},
			Spec: corev1.ServiceSpec{ClusterIP: "10.96.0.12"},
		},
		"scylladb-dc-ad-3-0": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "scylladb-dc-ad-3-0",
				Namespace: "sophena",
				Labels: map[string]string{
					naming.ScyllaServiceTypeLabel: string(naming.ScyllaServiceTypeMember),
				},
			},
			Spec: corev1.ServiceSpec{ClusterIP: "10.96.0.13"},
		},
	}

	dnsNames, err := scyllaServingDNSNamesForServices(serviceMap, "cluster.local")
	if err != nil {
		t.Fatalf("can't collect serving DNS names: %v", err)
	}
	expected := []string{
		"scylladb-client.sophena.svc",
		"scylladb-client.sophena.svc.cluster.local",
		"scylladb-dc-ad-1-0.sophena.svc",
		"scylladb-dc-ad-1-0.sophena.svc.cluster.local",
		"scylladb-dc-ad-2-0.sophena.svc",
		"scylladb-dc-ad-2-0.sophena.svc.cluster.local",
		"scylladb-dc-ad-3-0.sophena.svc",
		"scylladb-dc-ad-3-0.sophena.svc.cluster.local",
	}
	if !reflect.DeepEqual(dnsNames, expected) {
		t.Fatalf("expected serving SANs %v, got %v", expected, dnsNames)
	}

	ipAddresses, err := scyllaServingServiceIPAddresses(serviceMap)
	if err != nil {
		t.Fatalf("can't collect serving Service IP addresses: %v", err)
	}
	gotIPs := make([]string, 0, len(ipAddresses))
	for _, ipAddress := range ipAddresses {
		gotIPs = append(gotIPs, ipAddress.String())
	}
	expectedIPs := []string{"10.96.0.10", "10.96.0.11", "10.96.0.12", "10.96.0.13"}
	if !reflect.DeepEqual(gotIPs, expectedIPs) {
		t.Fatalf("expected identity and every member Service IP SAN %v, got %v", expectedIPs, gotIPs)
	}
}

func TestAgentServingCertificateUsesConfiguredClusterDomain(t *testing.T) {
	t.Parallel()

	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	if err := indexer.Add(&scyllav1alpha1.ScyllaOperatorConfig{
		ObjectMeta: metav1.ObjectMeta{Name: naming.SingletonName},
		Spec: scyllav1alpha1.ScyllaOperatorConfigSpec{
			ConfiguredClusterDomain: pointer.Ptr("custom.cluster.example."),
		},
		Status: scyllav1alpha1.ScyllaOperatorConfigStatus{
			ClusterDomain: pointer.Ptr("discovered.invalid"),
		},
	}); err != nil {
		t.Fatalf("can't add ScyllaOperatorConfig to indexer: %v", err)
	}
	controller := &Controller{
		scyllaOperatorConfigLister: scyllav1alpha1listers.NewScyllaOperatorConfigLister(indexer),
	}
	domain, err := controller.clusterDomain()
	if err != nil {
		t.Fatalf("can't resolve configured cluster domain: %v", err)
	}
	if domain != "custom.cluster.example" {
		t.Fatalf("expected configured cluster domain without trailing dot, got %q", domain)
	}
}
