// Copyright (C) 2026 ScyllaDB

package managerclientsecure

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateClusterSecureFields(t *testing.T) {
	t.Parallel()

	var request map[string]interface{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/clusters" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("can't decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Location", "/api/v1/cluster/id-1")
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client, err := NewClient(server.URL+"/api/v1", server.Client())
	if err != nil {
		t.Fatalf("can't create client: %v", err)
	}
	id, err := client.CreateCluster(context.Background(), &Cluster{
		Name:                      "ScyllaCluster/scylladb",
		Host:                      "scylladb-client.sophena.svc",
		AuthToken:                 "agent-token",
		Username:                  "cql-user",
		Password:                  "cql-password",
		AlternatorAccessKeyID:     "alternator-key",
		AlternatorSecretAccessKey: "alternator-secret",
		CQLCAFile:                 []byte("cql-ca"),
		CQLServerName:             "scylladb-client.sophena.svc",
		AlternatorCAFile:          []byte("alternator-ca"),
		AlternatorServerName:      "scylladb-client.sophena.svc",
		AgentCAFile:               []byte("agent-ca"),
		AgentServerName:           "scylladb-client.sophena.svc",
	})
	if err != nil {
		t.Fatalf("can't create cluster: %v", err)
	}
	if id != "id-1" {
		t.Fatalf("expected cluster ID id-1, got %q", id)
	}

	for _, field := range []string{
		"auth_token", "username", "password", "alternator_access_key_id", "alternator_secret_access_key",
		"cql_ca_file", "cql_server_name", "alternator_ca_file", "alternator_server_name", "agent_ca_file", "agent_server_name",
	} {
		if _, found := request[field]; !found {
			t.Errorf("secure Manager request is missing %q", field)
		}
	}
	for _, forbidden := range []string{"force_tls_disabled", "force_non_ssl_session_port"} {
		if _, found := request[forbidden]; found {
			t.Errorf("secure Manager request must not contain %q", forbidden)
		}
	}
}

func TestClientRequiresHTTPSAndDoesNotExposeErrorBody(t *testing.T) {
	t.Parallel()

	if _, err := NewClient("http://manager.example/api/v1", http.DefaultClient); err == nil {
		t.Fatal("expected plain HTTP Manager URL to be rejected")
	}
	if _, err := NewClient("https://manager.example/api/v1", nil); err == nil {
		t.Fatal("expected a missing verified Manager HTTP client to be rejected")
	}
	errorValue := (&HTTPError{Method: http.MethodPost, URL: "https://manager.example", StatusCode: http.StatusBadRequest}).Error()
	if errorValue != "Manager API POST https://manager.example returned HTTP 400" {
		t.Fatalf("unexpected redacted error: %q", errorValue)
	}
}
