// Copyright (C) 2026 ScyllaDB

package managerclientsecure

import (
	"encoding/json"
	"testing"

	"github.com/go-openapi/strfmt"
	managerapimodels "github.com/scylladb/scylla-manager/v3/swagger/gen/scylla-manager/models"
)

// TestCompanionGeneratedManagerAPIContract is a compile- and wire-format gate
// for the remotely pinned companion Manager modules. The controller keeps its
// narrow compatibility client until the additions are released upstream, but
// vendoring must still prove that the reviewed generated API contains every
// field used by that compatibility layer.
func TestCompanionGeneratedManagerAPIContract(t *testing.T) {
	t.Parallel()

	cluster := &managerapimodels.Cluster{
		AuthToken:                 "agent-token",
		Username:                  "cql-user",
		Password:                  "cql-password",
		AlternatorAccessKeyID:     "alternator-key",
		AlternatorSecretAccessKey: "alternator-secret",
		CqlCaFile:                 strfmt.Base64([]byte("cql-ca")),
		CqlServerName:             "scylladb-client.sophena.svc",
		SslUserCertFile:           strfmt.Base64([]byte("cql-client-cert")),
		SslUserKeyFile:            strfmt.Base64([]byte("cql-client-key")),
		AlternatorCaFile:          strfmt.Base64([]byte("alternator-ca")),
		AlternatorServerName:      "scylladb-client.sophena.svc",
		AgentCaFile:               strfmt.Base64([]byte("agent-ca")),
		AgentServerName:           "scylladb-client.sophena.svc",
	}

	data, err := json.Marshal(cluster)
	if err != nil {
		t.Fatalf("can't marshal companion generated Cluster: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatalf("can't inspect companion generated Cluster JSON: %v", err)
	}
	for _, field := range []string{
		"auth_token", "username", "password",
		"alternator_access_key_id", "alternator_secret_access_key",
		"cql_ca_file", "cql_server_name", "ssl_user_cert_file", "ssl_user_key_file",
		"alternator_ca_file", "alternator_server_name", "agent_ca_file", "agent_server_name",
	} {
		if _, found := fields[field]; !found {
			t.Errorf("companion generated Cluster is missing JSON field %q", field)
		}
	}

	sanitized := &managerapimodels.Cluster{
		AuthTokenSet:             true,
		CqlCredentialsSet:        true,
		AlternatorCredentialsSet: true,
		CqlCaSet:                 true,
		AlternatorCaSet:          true,
		AgentCaSet:               true,
		SslUserCertSet:           true,
	}
	sanitizedData, err := json.Marshal(sanitized)
	if err != nil {
		t.Fatalf("can't marshal companion generated sanitized Cluster fields: %v", err)
	}
	fields = nil
	if err := json.Unmarshal(sanitizedData, &fields); err != nil {
		t.Fatalf("can't inspect companion generated sanitized Cluster JSON: %v", err)
	}
	for _, field := range []string{
		"auth_token_set", "cql_credentials_set", "alternator_credentials_set",
		"cql_ca_set", "alternator_ca_set", "agent_ca_set", "ssl_user_cert_set",
	} {
		if _, found := fields[field]; !found {
			t.Errorf("companion generated sanitized Cluster contract is missing JSON field %q", field)
		}
	}

	status := &managerapimodels.ClusterStatusItems0{
		CqlTLSVerified:         true,
		CqlAuthVerified:        true,
		AlternatorTLSVerified:  true,
		AlternatorAuthVerified: true,
		AgentTLSVerified:       true,
	}
	statusData, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("can't marshal companion generated ClusterStatus item: %v", err)
	}
	fields = nil
	if err := json.Unmarshal(statusData, &fields); err != nil {
		t.Fatalf("can't inspect companion generated ClusterStatus JSON: %v", err)
	}
	for _, field := range []string{
		"cql_tls_verified", "cql_auth_verified",
		"alternator_tls_verified", "alternator_auth_verified", "agent_tls_verified",
	} {
		if _, found := fields[field]; !found {
			t.Errorf("companion generated ClusterStatus is missing JSON field %q", field)
		}
	}
}
