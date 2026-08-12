// Copyright (C) 2026 ScyllaDB

// Package managerclientsecure contains the narrow compatibility surface needed by the
// secure cluster-registration controller. It can be replaced by the generated Manager
// client after the companion secure-cluster-tls Manager API is published.
package managerclientsecure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
)

type Cluster struct {
	ID                        string            `json:"id,omitempty"`
	Name                      string            `json:"name,omitempty"`
	Host                      string            `json:"host,omitempty"`
	AuthToken                 string            `json:"auth_token,omitempty"`
	Username                  string            `json:"username,omitempty"`
	Password                  string            `json:"password,omitempty"`
	AlternatorAccessKeyID     string            `json:"alternator_access_key_id,omitempty"`
	AlternatorSecretAccessKey string            `json:"alternator_secret_access_key,omitempty"`
	CQLCAFile                 []byte            `json:"cql_ca_file,omitempty"`
	CQLServerName             string            `json:"cql_server_name,omitempty"`
	SSLUserCertFile           []byte            `json:"ssl_user_cert_file,omitempty"`
	SSLUserKeyFile            []byte            `json:"ssl_user_key_file,omitempty"`
	AlternatorCAFile          []byte            `json:"alternator_ca_file,omitempty"`
	AlternatorServerName      string            `json:"alternator_server_name,omitempty"`
	AgentCAFile               []byte            `json:"agent_ca_file,omitempty"`
	AgentServerName           string            `json:"agent_server_name,omitempty"`
	Labels                    map[string]string `json:"labels,omitempty"`
	WithoutRepair             bool              `json:"without_repair,omitempty"`
}

type ClusterStatusItem struct {
	Host                   string `json:"host,omitempty"`
	CQLTLSVerified         bool   `json:"cql_tls_verified,omitempty"`
	CQLAuthVerified        bool   `json:"cql_auth_verified,omitempty"`
	AlternatorTLSVerified  bool   `json:"alternator_tls_verified,omitempty"`
	AlternatorAuthVerified bool   `json:"alternator_auth_verified,omitempty"`
	AgentTLSVerified       bool   `json:"agent_tls_verified,omitempty"`
}

type HTTPError struct {
	Method     string
	URL        string
	StatusCode int
}

func (e *HTTPError) Error() string {
	// Manager error bodies are deliberately excluded because an upstream validation
	// response could echo credential-bearing request fields into controller logs/events.
	return fmt.Sprintf("Manager API %s %s returned HTTP %d", e.Method, e.URL, e.StatusCode)
}

type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
}

func NewClient(rawURL string, httpClient *http.Client) (*Client, error) {
	baseURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("can't parse Manager API URL: %w", err)
	}
	if baseURL.Scheme != "https" || baseURL.Host == "" || baseURL.User != nil || baseURL.RawQuery != "" || baseURL.Fragment != "" {
		return nil, fmt.Errorf("Manager API URL must be an absolute https URL without user information, query, or fragment")
	}
	if httpClient == nil {
		return nil, fmt.Errorf("verified Manager API HTTP client must not be nil")
	}

	return &Client{baseURL: baseURL, httpClient: httpClient}, nil
}

func (c *Client) CreateCluster(ctx context.Context, cluster *Cluster) (string, error) {
	response, err := c.doJSON(ctx, http.MethodPost, "clusters", cluster)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	location := response.Header.Get("Location")
	if len(location) == 0 {
		return "", fmt.Errorf("Manager create-cluster response is missing Location header")
	}
	locationURL, err := url.Parse(location)
	if err != nil {
		return "", fmt.Errorf("can't parse Manager create-cluster Location header: %w", err)
	}
	clusterID := path.Base(strings.TrimSuffix(locationURL.Path, "/"))
	if len(clusterID) == 0 || clusterID == "." || clusterID == "/" {
		return "", fmt.Errorf("Manager create-cluster Location header does not contain a cluster ID")
	}

	return clusterID, nil
}

func (c *Client) UpdateCluster(ctx context.Context, cluster *Cluster) error {
	if len(cluster.ID) == 0 {
		return fmt.Errorf("cluster ID must not be empty")
	}

	response, err := c.doJSON(ctx, http.MethodPut, "cluster/"+url.PathEscape(cluster.ID), cluster)
	if err != nil {
		return err
	}
	return response.Body.Close()
}

func (c *Client) GetCluster(ctx context.Context, clusterID string) (*Cluster, error) {
	response, err := c.doJSON(ctx, http.MethodGet, "cluster/"+url.PathEscape(clusterID), nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var cluster Cluster
	if err := json.NewDecoder(response.Body).Decode(&cluster); err != nil {
		return nil, fmt.Errorf("can't decode Manager cluster: %w", err)
	}
	return &cluster, nil
}

func (c *Client) ListClusters(ctx context.Context) ([]*Cluster, error) {
	response, err := c.doJSON(ctx, http.MethodGet, "clusters", nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var clusters []*Cluster
	if err := json.NewDecoder(response.Body).Decode(&clusters); err != nil {
		return nil, fmt.Errorf("can't decode Manager clusters: %w", err)
	}
	return clusters, nil
}

func (c *Client) DeleteCluster(ctx context.Context, clusterID string) error {
	response, err := c.doJSON(ctx, http.MethodDelete, "cluster/"+url.PathEscape(clusterID), nil)
	if err != nil {
		return err
	}
	return response.Body.Close()
}

func IsNotFound(err error) bool {
	httpError, ok := err.(*HTTPError)
	return ok && httpError.StatusCode == http.StatusNotFound
}

func (c *Client) ClusterStatus(ctx context.Context, clusterID string) ([]*ClusterStatusItem, error) {
	response, err := c.doJSON(ctx, http.MethodGet, "cluster/"+url.PathEscape(clusterID)+"/status", nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var status []*ClusterStatusItem
	if err := json.NewDecoder(response.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("can't decode Manager cluster status: %w", err)
	}
	return status, nil
}

func (c *Client) doJSON(ctx context.Context, method, relativePath string, requestBody interface{}) (*http.Response, error) {
	requestURL := *c.baseURL
	requestURL.Path = path.Join(c.baseURL.Path, relativePath)

	var body io.Reader
	if requestBody != nil {
		var encoded bytes.Buffer
		if err := json.NewEncoder(&encoded).Encode(requestBody); err != nil {
			return nil, fmt.Errorf("can't encode Manager API request: %w", err)
		}
		body = &encoded
	}

	request, err := http.NewRequestWithContext(ctx, method, requestURL.String(), body)
	if err != nil {
		return nil, fmt.Errorf("can't build Manager API request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	if requestBody != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("Manager API request failed: %w", err)
	}
	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		return response, nil
	}
	defer response.Body.Close()

	return nil, &HTTPError{
		Method:     method,
		URL:        requestURL.Redacted(),
		StatusCode: response.StatusCode,
	}
}
