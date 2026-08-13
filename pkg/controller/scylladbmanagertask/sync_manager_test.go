// Copyright (C) 2025 ScyllaDB

package scylladbmanagertask

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/scylladb/scylla-manager/v3/pkg/managerclient"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav1alpha1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllertools"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
)

var (
	validTimeValue = "2024-03-20T14:49:33.590Z"
	validTime      = helpers.Must(time.Parse(time.RFC3339, validTimeValue))
)

func Test_makeRequiredScyllaDBManagerClientTask(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name            string
		smt             *scyllav1alpha1.ScyllaDBManagerTask
		clusterID       string
		overrideOptions []scyllaDBManagerClientTaskOverrideOption
		expected        *managerclient.Task
		expectedErr     error
	}{
		{
			name:            "basic backup",
			smt:             newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef(),
			clusterID:       "cluster-id",
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": "UEsXaoGEh/QtOTqgFiII3MJh25u0Sxob4eNsTnpztdAKXy9SOg22FUtWBDBclQ4rIrUluxyO6cjAalit3uEDcg==",
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "backup",
				Properties: map[string]any{
					"dc":                []string{"dc1", "!otherdc*"},
					"keyspace":          []string{"keyspace", "!keyspace.table_prefix_*"},
					"location":          []string{"gcs:test"},
					"rate_limit":        []string{"dc1:1", "2"},
					"retention":         pointer.Ptr[int64](3),
					"snapshot_parallel": []string{"dc1:2", "3"},
					"upload_parallel":   []string{"dc1:3", "4"},
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "1m0s",
					StartDate:  pointer.Ptr(strfmt.DateTime(validTime)),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "backup",
			},
			expectedErr: nil,
		},
		{
			name:            "basic repair",
			smt:             newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef(),
			clusterID:       "cluster-id",
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": "LBUu5lyb5qZl+sZ9B8Clgs/mhBPX7MPv95MvvqAD4g0x4RFKkSr4vYzUiHQTsyXzM8+WXKpBkDEIM9O6RmQhog==",
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "repair",
				Properties: map[string]any{
					"dc":                    []string{"dc1", "!otherdc*"},
					"keyspace":              []string{"keyspace", "!keyspace.table_prefix_*"},
					"fail_fast":             true,
					"host":                  "10.0.0.1",
					"ignore_down_hosts":     false,
					"intensity":             float64(1),
					"parallel":              int64(1),
					"small_table_threshold": int64(1073741824),
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "1m0s",
					StartDate:  pointer.Ptr(strfmt.DateTime(validTime)),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "repair",
			},
			expectedErr: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := makeScyllaDBManagerClientTask(tc.smt, tc.clusterID, tc.overrideOptions...)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected and got errors differ:\n%s\n", cmp.Diff(tc.expectedErr, err, cmpopts.EquateErrors()))
			}

			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("expected and got ScyllaDB Manager client tasks differ:\n%s\n", cmp.Diff(tc.expected, got))
			}
		})
	}
}

func TestSyncManagerBlocksManagerAPICallsUntilCurrentVerification(t *testing.T) {
	t.Parallel()

	const registrationGeneration int64 = 2
	clusterID := "cluster-id"
	trueCondition := func(conditionType string, observedGeneration int64) metav1.Condition {
		return metav1.Condition{
			Type:               conditionType,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: observedGeneration,
			Reason:             "Verified",
		}
	}
	falseCondition := func(conditionType string) metav1.Condition {
		return metav1.Condition{
			Type:               conditionType,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: registrationGeneration,
			Reason:             "NotVerified",
		}
	}

	tests := []struct {
		name       string
		conditions []metav1.Condition
	}{
		{name: "conditions missing"},
		{
			name: "Manager API condition false",
			conditions: []metav1.Condition{
				falseCondition(scyllav1alpha1.ScyllaDBManagerClusterRegistrationManagerAPIConnectionVerifiedCondition),
				trueCondition(scyllav1alpha1.ScyllaDBManagerClusterRegistrationDatabaseConnectionVerifiedCondition, registrationGeneration),
			},
		},
		{
			name: "Manager API condition stale",
			conditions: []metav1.Condition{
				trueCondition(scyllav1alpha1.ScyllaDBManagerClusterRegistrationManagerAPIConnectionVerifiedCondition, registrationGeneration-1),
				trueCondition(scyllav1alpha1.ScyllaDBManagerClusterRegistrationDatabaseConnectionVerifiedCondition, registrationGeneration),
			},
		},
		{
			name: "database condition missing",
			conditions: []metav1.Condition{
				trueCondition(scyllav1alpha1.ScyllaDBManagerClusterRegistrationManagerAPIConnectionVerifiedCondition, registrationGeneration),
			},
		},
		{
			name: "database condition false",
			conditions: []metav1.Condition{
				trueCondition(scyllav1alpha1.ScyllaDBManagerClusterRegistrationManagerAPIConnectionVerifiedCondition, registrationGeneration),
				falseCondition(scyllav1alpha1.ScyllaDBManagerClusterRegistrationDatabaseConnectionVerifiedCondition),
			},
		},
		{
			name: "database condition stale",
			conditions: []metav1.Condition{
				trueCondition(scyllav1alpha1.ScyllaDBManagerClusterRegistrationManagerAPIConnectionVerifiedCondition, registrationGeneration),
				trueCondition(scyllav1alpha1.ScyllaDBManagerClusterRegistrationDatabaseConnectionVerifiedCondition, registrationGeneration-1),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			smt := newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef()
			smt.Spec.ScyllaDBClusterRef.Kind = scyllav1.ScyllaClusterGVK.Kind
			smcrName, err := naming.ScyllaDBManagerClusterRegistrationNameForScyllaDBManagerTask(smt)
			if err != nil {
				t.Fatalf("can't calculate registration name: %v", err)
			}
			smcr := &scyllav1alpha1.ScyllaDBManagerClusterRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name:       smcrName,
					Namespace:  smt.Namespace,
					Generation: registrationGeneration,
				},
				Spec: scyllav1alpha1.ScyllaDBManagerClusterRegistrationSpec{
					ScyllaDBClusterRef: smt.Spec.ScyllaDBClusterRef,
				},
				Status: scyllav1alpha1.ScyllaDBManagerClusterRegistrationStatus{
					ClusterID:  &clusterID,
					Conditions: tc.conditions,
				},
			}

			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			if err := indexer.Add(smcr); err != nil {
				t.Fatalf("can't add registration to indexer: %v", err)
			}
			kubeClient := fake.NewSimpleClientset()
			controller := &Controller{
				kubeClient:                               kubeClient,
				scyllaDBManagerClusterRegistrationLister: scyllav1alpha1listers.NewScyllaDBManagerClusterRegistrationLister(indexer),
			}

			conditions, err := controller.syncManager(context.Background(), smt, &scyllav1alpha1.ScyllaDBManagerTaskStatus{})
			if err != nil {
				t.Fatalf("expected task to wait without error, got: %v", err)
			}
			if len(conditions) != 1 || conditions[0].Reason != "AwaitingVerifiedScyllaDBManagerClusterRegistration" {
				t.Fatalf("unexpected progressing conditions: %#v", conditions)
			}
			if actions := kubeClient.Actions(); len(actions) != 0 {
				t.Fatalf("expected no Kubernetes or Manager-client setup calls before verification, got actions: %#v", actions)
			}
		})
	}
}

func TestScyllaDBManagerClusterRegistrationReadyForTasks(t *testing.T) {
	t.Parallel()

	smcr := &scyllav1alpha1.ScyllaDBManagerClusterRegistration{
		ObjectMeta: metav1.ObjectMeta{Generation: 3},
		Status: scyllav1alpha1.ScyllaDBManagerClusterRegistrationStatus{
			Conditions: []metav1.Condition{
				{Type: scyllav1alpha1.ScyllaDBManagerClusterRegistrationManagerAPIConnectionVerifiedCondition, Status: metav1.ConditionTrue, ObservedGeneration: 3},
				{Type: scyllav1alpha1.ScyllaDBManagerClusterRegistrationDatabaseConnectionVerifiedCondition, Status: metav1.ConditionTrue, ObservedGeneration: 3},
			},
		},
	}
	if !scyllaDBManagerClusterRegistrationReadyForTasks(smcr) {
		t.Fatal("expected current verified conditions to allow Manager task reconciliation")
	}
}

func TestValidateBackupTaskCreateAndUpdateContract(t *testing.T) {
	t.Parallel()

	task := newValidateBackupScyllaDBManagerTask()
	required, err := makeScyllaDBManagerClientTask(task, "cluster-id")
	if err != nil {
		t.Fatalf("can't make validate_backup task: %v", err)
	}
	if required.Type != managerclient.ValidateBackupTask || required.Name != "weekly-backup-validation" {
		t.Fatalf("unexpected type/name: %q/%q", required.Type, required.Name)
	}
	properties := required.Properties.(map[string]any)
	if got, ok := properties["delete_orphaned_files"].(bool); !ok || got {
		t.Fatalf("validate_backup must always send delete_orphaned_files=false, got %#v", properties["delete_orphaned_files"])
	}
	if diff := cmp.Diff([]string{"s3:sophena-backups"}, properties["location"]); diff != "" {
		t.Fatalf("unexpected locations (-want,+got):\n%s", diff)
	}
	if required.Schedule.Timezone != "America/Vancouver" || required.Schedule.Cron != "0 3 * * SUN" || required.Schedule.NumRetries != 3 || required.Schedule.RetryWait != "10m0s" {
		t.Fatalf("unexpected schedule: %#v", required.Schedule)
	}

	updated := task.DeepCopy()
	updated.Spec.ValidateBackup.Location = []string{"s3:sophena-backups-2"}
	updatedRequired, err := makeScyllaDBManagerClientTask(updated, "cluster-id")
	if err != nil {
		t.Fatalf("can't make updated validate_backup task: %v", err)
	}
	if required.Labels[naming.ManagedHash] == updatedRequired.Labels[naming.ManagedHash] {
		t.Fatal("location drift must change the managed hash and trigger an in-place Manager update")
	}

	observed := &managerclient.TaskListItem{
		Enabled:  true,
		Labels:   maps.Clone(required.Labels),
		Name:     required.Name,
		Type:     required.Type,
		Schedule: required.Schedule,
		Properties: map[string]interface{}{
			"location":              []interface{}{"s3:sophena-backups"},
			"delete_orphaned_files": false,
		},
	}
	if !validateBackupManagerTaskMatchesDesired(observed, required) {
		t.Fatal("exact Manager readback should match desired validation task")
	}
	// Manager's list endpoint flattens a singleton location array to a scalar.
	// This normalized response is semantically identical and must converge.
	observed.Properties.(map[string]interface{})["location"] = "s3:sophena-backups"
	if !validateBackupManagerTaskMatchesDesired(observed, required) {
		t.Fatal("scalar Manager location readback should match singleton desired location")
	}
	// Manager's list endpoint also serializes the internal cron expression as
	// a JSON wrapper. It is semantically identical to the desired plain cron
	// and must not trigger an update every safety resync.
	observedSchedule := *observed.Schedule
	observed.Schedule = &observedSchedule
	observed.Schedule.Cron = `{"spec":"0 3 * * SUN","start_date":"2026-08-13T08:00:00Z"}`
	if !validateBackupManagerTaskMatchesDesired(observed, required) {
		t.Fatal("Manager cron wire wrapper should match the desired plain cron")
	}
	observed.Schedule.Cron = `{"spec":"0 4 * * SUN","start_date":"2026-08-13T08:00:00Z"}`
	if validateBackupManagerTaskMatchesDesired(observed, required) {
		t.Fatal("different cron spec in Manager wire wrapper must be detected")
	}
	observed.Schedule.Cron = required.Schedule.Cron
	// Keeping a stale managed-hash label must not hide an out-of-band property change.
	observed.Properties.(map[string]interface{})["location"] = "s3:drifted"
	if validateBackupManagerTaskMatchesDesired(observed, required) {
		t.Fatal("strict readback must detect location drift even when the managed-hash label is unchanged")
	}
}

func TestValidateBackupTaskAdoptExternalDeleteRestartFailureAndNoDuplicateResolution(t *testing.T) {
	t.Parallel()

	canonical := &managerclient.TaskListItem{
		ID:   "task-a",
		Name: "weekly-backup-validation",
		Type: managerclient.ValidateBackupTask,
		Properties: map[string]interface{}{
			"location":              []interface{}{"s3:sophena-backups"},
			"delete_orphaned_files": false,
		},
	}

	// Restart/adoption finds the sole canonical task even with no status task ID.
	got, found, err := resolveScyllaDBManagerClientTask([]*managerclient.TaskListItem{canonical}, managerclient.ValidateBackupTask, canonical.Name, "uid")
	if err != nil || !found || got.ID != canonical.ID || !isNonDestructiveValidateBackupTask(got) {
		t.Fatalf("safe restart/adoption resolution failed: found=%t got=%#v err=%v", found, got, err)
	}

	// External deletion produces an unambiguous not-found result, allowing exactly one recreation.
	got, found, err = resolveScyllaDBManagerClientTask(nil, managerclient.ValidateBackupTask, canonical.Name, "uid")
	if err != nil || found || got != nil {
		t.Fatalf("external deletion resolution failed: found=%t got=%#v err=%v", found, got, err)
	}

	// An unrelated task is ignored; two canonical tasks fail closed rather than creating a third.
	duplicateValue := *canonical
	duplicate := &duplicateValue
	duplicate.ID = "task-b"
	_, _, err = resolveScyllaDBManagerClientTask([]*managerclient.TaskListItem{canonical, duplicate}, managerclient.ValidateBackupTask, canonical.Name, "uid")
	if err == nil || !controllertools.IsNonRetriable(err) {
		t.Fatalf("expected duplicate canonical tasks to fail closed, got %v", err)
	}

	renamed := *canonical
	renamed.Name = "externally-renamed"
	renamed.Labels = map[string]string{naming.OwnerUIDLabel: "uid"}
	got, found, err = resolveScyllaDBManagerClientTask([]*managerclient.TaskListItem{&renamed}, managerclient.ValidateBackupTask, canonical.Name, "uid")
	if err != nil || !found || got.ID != canonical.ID {
		t.Fatalf("sole externally renamed owned task must update in place: found=%t got=%#v err=%v", found, got, err)
	}

	destructiveValue := *canonical
	destructive := &destructiveValue
	destructive.Properties = map[string]interface{}{"delete_orphaned_files": true}
	if isNonDestructiveValidateBackupTask(destructive) {
		t.Fatal("destructive task must never be considered safe for adoption")
	}
	malformedValue := *canonical
	malformed := &malformedValue
	malformed.Properties = "invalid"
	if isNonDestructiveValidateBackupTask(malformed) {
		t.Fatal("malformed properties must fail closed")
	}
}

func TestValidateBackupTaskStrictReadbackAndOwnedDelete(t *testing.T) {
	t.Parallel()

	next := strfmt.DateTime(validTime.Add(time.Hour))
	lastSuccess := strfmt.DateTime(validTime)
	lastError := strfmt.DateTime(validTime.Add(-time.Hour))
	managerTask := &managerclient.TaskListItem{
		ID:             "task-id",
		Name:           "weekly-backup-validation",
		Status:         managerclient.TaskStatusDone,
		NextActivation: &next,
		LastSuccess:    &lastSuccess,
		LastError:      &lastError,
		Labels:         map[string]string{naming.OwnerUIDLabel: "uid"},
	}
	owner := newValidateBackupScyllaDBManagerTask()
	owner.Generation = 7
	status := (&Controller{}).calculateStatus(owner)
	syncScyllaDBManagerTaskReadbackStatus(status, managerTask)
	if status.TaskID == nil || *status.TaskID != "task-id" || status.ManagerStatus == nil || *status.ManagerStatus != managerclient.TaskStatusDone {
		t.Fatalf("incomplete ID/status readback: %#v", status)
	}
	if status.NextActivation == nil || !status.NextActivation.Time.Equal(time.Time(next)) || status.LastSuccess == nil || status.LastError == nil {
		t.Fatalf("incomplete activation readback: %#v", status)
	}
	if status.ObservedGeneration == nil || *status.ObservedGeneration != 7 {
		t.Fatalf("observed generation was not preserved in strict readback: %#v", status)
	}

	if !isScyllaDBManagerTaskOwnedBy(managerTask, owner) {
		t.Fatal("exact owner UID must permit finalizer deletion")
	}
	managerTask.Labels[naming.OwnerUIDLabel] = "different-uid"
	if isScyllaDBManagerTaskOwnedBy(managerTask, owner) {
		t.Fatal("mismatched owner UID must prohibit finalizer deletion")
	}
}

func TestValidateBackupTaskCreateUsesManagerAPIAndFailsClosed(t *testing.T) {
	t.Parallel()

	var requestBody map[string]interface{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/cluster/cluster-id/tasks" {
			t.Errorf("unexpected Manager request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Errorf("can't decode Manager task request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Location", "/api/v1/cluster/cluster-id/task/validate_backup/11111111-1111-4111-8111-111111111111")
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	transport, ok := server.Client().Transport.(*http.Transport)
	if !ok || transport.TLSClientConfig == nil {
		t.Fatal("test server did not provide TLS configuration")
	}
	managerClient, err := managerclient.NewClient(server.URL+"/api/v1", managerclient.WithTLSConfig(transport.TLSClientConfig.Clone()))
	if err != nil {
		t.Fatalf("can't create Manager client: %v", err)
	}
	// Assert this test remains a verified TLS request even if httptest's defaults change.
	if transport.TLSClientConfig.MinVersion != 0 && transport.TLSClientConfig.MinVersion < tls.VersionTLS12 {
		t.Fatal("test Manager transport permits obsolete TLS")
	}

	status := &scyllav1alpha1.ScyllaDBManagerTaskStatus{}
	controller := &Controller{}
	conditions, err := controller.syncManagerClientTaskNotFound(context.Background(), newValidateBackupScyllaDBManagerTask(), status, &managerClient, "cluster-id")
	if err != nil {
		t.Fatalf("can't create Manager validation task: %v", err)
	}
	if len(conditions) != 1 || conditions[0].Reason != "CreatedScyllaDBManagerTask" || status.TaskID == nil || *status.TaskID != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("unexpected create result: conditions=%#v status=%#v", conditions, status)
	}
	properties, ok := requestBody["properties"].(map[string]interface{})
	if !ok || properties["delete_orphaned_files"] != false {
		t.Fatalf("Manager create request was not provably non-destructive: %#v", requestBody)
	}
}

func TestValidateBackupTaskManagerCreateFailureDoesNotClaimOwnership(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"injected failure"}`))
	}))
	defer server.Close()

	transport := server.Client().Transport.(*http.Transport)
	managerClient, err := managerclient.NewClient(server.URL+"/api/v1", managerclient.WithTLSConfig(transport.TLSClientConfig.Clone()))
	if err != nil {
		t.Fatalf("can't create Manager client: %v", err)
	}
	status := &scyllav1alpha1.ScyllaDBManagerTaskStatus{}
	controller := &Controller{}
	conditions, err := controller.syncManagerClientTaskNotFound(context.Background(), newValidateBackupScyllaDBManagerTask(), status, &managerClient, "cluster-id")
	if err == nil {
		t.Fatal("expected Manager create failure")
	}
	if status.TaskID != nil || len(conditions) != 0 {
		t.Fatalf("failed create must not claim a Manager task: conditions=%#v status=%#v", conditions, status)
	}
}

func newValidateBackupScyllaDBManagerTask() *scyllav1alpha1.ScyllaDBManagerTask {
	cron := "0 3 * * SUN"
	timezone := "America/Vancouver"
	return &scyllav1alpha1.ScyllaDBManagerTask{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "weekly-backup-validation",
			Namespace: "sophena",
			UID:       "uid",
		},
		Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
			ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{Name: "scylladb", Kind: scyllav1alpha1.ScyllaDBDatacenterGVK.Kind},
			Type:               scyllav1alpha1.ScyllaDBManagerTaskTypeValidateBackup,
			ValidateBackup: &scyllav1alpha1.ScyllaDBManagerValidateBackupTaskOptions{
				ScyllaDBManagerTaskSchedule: scyllav1alpha1.ScyllaDBManagerTaskSchedule{
					Cron:       &cron,
					Timezone:   &timezone,
					NumRetries: pointer.Ptr[int64](3),
					RetryWait:  &metav1.Duration{Duration: 10 * time.Minute},
					StartDate:  &metav1.Time{Time: validTime},
				},
				Location: []string{"s3:sophena-backups"},
			},
		},
	}
}

func Test_makeRequiredScyllaDBManagerClientTaskWithManagedHashFunc(t *testing.T) {
	t.Parallel()

	const mockManagedHash = "mock-managed-hash"
	getMockManagedHash := func(_ *managerclient.Task) (string, error) {
		return mockManagedHash, nil
	}

	tt := []struct {
		name            string
		smt             *scyllav1alpha1.ScyllaDBManagerTask
		clusterID       string
		managedHashFunc func(*managerclient.Task) (string, error)
		overrideOptions []scyllaDBManagerClientTaskOverrideOption
		expected        *managerclient.Task
		expectedErr     error
	}{
		{
			name:            "backup, sdc ref",
			smt:             newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "backup",
				Properties: map[string]any{
					"dc":                []string{"dc1", "!otherdc*"},
					"keyspace":          []string{"keyspace", "!keyspace.table_prefix_*"},
					"location":          []string{"gcs:test"},
					"rate_limit":        []string{"dc1:1", "2"},
					"retention":         pointer.Ptr[int64](3),
					"snapshot_parallel": []string{"dc1:2", "3"},
					"upload_parallel":   []string{"dc1:3", "4"},
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "1m0s",
					StartDate:  pointer.Ptr(strfmt.DateTime(validTime)),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "backup",
			},
			expectedErr: nil,
		},
		{
			name: "backup, sdc ref, without optional fields",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				smt.Spec.Backup = &scyllav1alpha1.ScyllaDBManagerBackupTaskOptions{
					Location: []string{"s3:test"},
				}

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "backup",
				Properties: map[string]any{
					"location": []string{"s3:test"},
				},
				Schedule: &managerclient.Schedule{},
				Tags:     nil,
				Type:     "backup",
			},
			expectedErr: nil,
		},
		{
			name: "backup, sdc ref, with name override annotation",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskNameOverrideAnnotation, "override")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "override",
				Properties: map[string]any{
					"dc":                []string{"dc1", "!otherdc*"},
					"keyspace":          []string{"keyspace", "!keyspace.table_prefix_*"},
					"location":          []string{"gcs:test"},
					"rate_limit":        []string{"dc1:1", "2"},
					"retention":         pointer.Ptr[int64](3),
					"snapshot_parallel": []string{"dc1:2", "3"},
					"upload_parallel":   []string{"dc1:3", "4"},
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "1m0s",
					StartDate:  pointer.Ptr(strfmt.DateTime(validTime)),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "backup",
			},
			expectedErr: nil,
		},
		{
			name: "backup, sdc ref, with interval override annotation",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleIntervalOverrideAnnotation, "7d")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "backup",
				Properties: map[string]any{
					"dc":                []string{"dc1", "!otherdc*"},
					"keyspace":          []string{"keyspace", "!keyspace.table_prefix_*"},
					"location":          []string{"gcs:test"},
					"rate_limit":        []string{"dc1:1", "2"},
					"retention":         pointer.Ptr[int64](3),
					"snapshot_parallel": []string{"dc1:2", "3"},
					"upload_parallel":   []string{"dc1:3", "4"},
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "7d",
					NumRetries: 3,
					RetryWait:  "1m0s",
					StartDate:  pointer.Ptr(strfmt.DateTime(validTime)),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "backup",
			},
			expectedErr: nil,
		},
		{
			name: "backup, sdc ref, with valid schedule startDate override annotation",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation, "2025-05-08T17:24:00.000Z")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "backup",
				Properties: map[string]any{
					"dc":                []string{"dc1", "!otherdc*"},
					"keyspace":          []string{"keyspace", "!keyspace.table_prefix_*"},
					"location":          []string{"gcs:test"},
					"rate_limit":        []string{"dc1:1", "2"},
					"retention":         pointer.Ptr[int64](3),
					"snapshot_parallel": []string{"dc1:2", "3"},
					"upload_parallel":   []string{"dc1:3", "4"},
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "1m0s",
					StartDate:  pointer.Ptr(helpers.Must(strfmt.ParseDateTime("2025-05-08T17:24:00.000Z"))),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "backup",
			},
			expectedErr: nil,
		},
		{
			name: "backup, sdc ref, with invalid schedule startDate override annotation",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation, "now+invalid")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: nil,
			expected:        nil,
			expectedErr:     fmt.Errorf("can't make ScyllaDB Manager client schedule: %w", apimachineryutilerrors.NewAggregate([]error{fmt.Errorf("can't parse start date: %w", fmt.Errorf("can't parse duration: %w", errors.New("time: invalid duration +invalid")))})),
		},
		{
			name: "backup, sdc ref, with invalid schedule startDate override annotation and start date retention override option",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation, "now+invalid")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: []scyllaDBManagerClientTaskOverrideOption{
				withScheduleStartDateNowSyntaxRetention(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
			},
			expected:    nil,
			expectedErr: fmt.Errorf("can't make ScyllaDB Manager client schedule: %w", apimachineryutilerrors.NewAggregate([]error{fmt.Errorf("can't parse start date: %w", fmt.Errorf("can't parse duration: %w", errors.New("time: invalid duration +invalid")))})),
		},
		{
			name: "backup, sdc ref, with 'now' schedule startDate override annotation and start date retention override option",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation, "now")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: []scyllaDBManagerClientTaskOverrideOption{
				withScheduleStartDateNowSyntaxRetention(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
			},
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "backup",
				Properties: map[string]any{
					"dc":                []string{"dc1", "!otherdc*"},
					"keyspace":          []string{"keyspace", "!keyspace.table_prefix_*"},
					"location":          []string{"gcs:test"},
					"rate_limit":        []string{"dc1:1", "2"},
					"retention":         pointer.Ptr[int64](3),
					"snapshot_parallel": []string{"dc1:2", "3"},
					"upload_parallel":   []string{"dc1:3", "4"},
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "1m0s",
					StartDate:  pointer.Ptr(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "backup",
			},
			expectedErr: nil,
		},
		{
			name: "backup, sdc ref, with 'now+duration' schedule startDate override annotation and start date retention override option",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation, "now+3d2h10m")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: []scyllaDBManagerClientTaskOverrideOption{
				withScheduleStartDateNowSyntaxRetention(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
			},
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "backup",
				Properties: map[string]any{
					"dc":                []string{"dc1", "!otherdc*"},
					"keyspace":          []string{"keyspace", "!keyspace.table_prefix_*"},
					"location":          []string{"gcs:test"},
					"rate_limit":        []string{"dc1:1", "2"},
					"retention":         pointer.Ptr[int64](3),
					"snapshot_parallel": []string{"dc1:2", "3"},
					"upload_parallel":   []string{"dc1:3", "4"},
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "1m0s",
					StartDate:  pointer.Ptr(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "backup",
			},
			expectedErr: nil,
		},
		{
			name: "backup, sdc ref, with valid time schedule startDate override annotation and start date retention override option",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation, "2025-05-08T17:24:00.000Z")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: []scyllaDBManagerClientTaskOverrideOption{
				withScheduleStartDateNowSyntaxRetention(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
			},
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "backup",
				Properties: map[string]any{
					"dc":                []string{"dc1", "!otherdc*"},
					"keyspace":          []string{"keyspace", "!keyspace.table_prefix_*"},
					"location":          []string{"gcs:test"},
					"rate_limit":        []string{"dc1:1", "2"},
					"retention":         pointer.Ptr[int64](3),
					"snapshot_parallel": []string{"dc1:2", "3"},
					"upload_parallel":   []string{"dc1:3", "4"},
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "1m0s",
					StartDate:  pointer.Ptr(helpers.Must(strfmt.ParseDateTime("2025-05-08T17:24:00.000Z"))),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "backup",
			},
			expectedErr: nil,
		},
		{
			name:            "backup, sdc ref, without startDate override annotation and with start date retention override option",
			smt:             newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: []scyllaDBManagerClientTaskOverrideOption{
				withScheduleStartDateNowSyntaxRetention(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
			},
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "backup",
				Properties: map[string]any{
					"dc":                []string{"dc1", "!otherdc*"},
					"keyspace":          []string{"keyspace", "!keyspace.table_prefix_*"},
					"location":          []string{"gcs:test"},
					"rate_limit":        []string{"dc1:1", "2"},
					"retention":         pointer.Ptr[int64](3),
					"snapshot_parallel": []string{"dc1:2", "3"},
					"upload_parallel":   []string{"dc1:3", "4"},
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "1m0s",
					StartDate:  pointer.Ptr(strfmt.DateTime(validTime)),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "backup",
			},
			expectedErr: nil,
		},
		{
			name: "backup, sdc ref, with schedule timezone override annotation",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleTimezoneOverrideAnnotation, "CET")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: []scyllaDBManagerClientTaskOverrideOption{
				withScheduleStartDateNowSyntaxRetention(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
			},
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "backup",
				Properties: map[string]any{
					"dc":                []string{"dc1", "!otherdc*"},
					"keyspace":          []string{"keyspace", "!keyspace.table_prefix_*"},
					"location":          []string{"gcs:test"},
					"rate_limit":        []string{"dc1:1", "2"},
					"retention":         pointer.Ptr[int64](3),
					"snapshot_parallel": []string{"dc1:2", "3"},
					"upload_parallel":   []string{"dc1:3", "4"},
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "1m0s",
					StartDate:  pointer.Ptr(strfmt.DateTime(validTime)),
					Timezone:   "CET",
					Window:     nil,
				},
				Tags: nil,
				Type: "backup",
			},
			expectedErr: nil,
		},
		{
			name:            "repair, sdc ref",
			smt:             newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "repair",
				Properties: map[string]any{
					"dc":                    []string{"dc1", "!otherdc*"},
					"keyspace":              []string{"keyspace", "!keyspace.table_prefix_*"},
					"fail_fast":             true,
					"host":                  "10.0.0.1",
					"ignore_down_hosts":     false,
					"intensity":             float64(1),
					"parallel":              int64(1),
					"small_table_threshold": int64(1073741824),
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "1m0s",
					StartDate:  pointer.Ptr(strfmt.DateTime(validTime)),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "repair",
			},
			expectedErr: nil,
		},
		{
			name: "repair, sdc ref, without optional fields",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				smt.Spec.Repair = &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{}

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name:       "repair",
				Properties: map[string]any{},
				Schedule:   &managerclient.Schedule{},
				Tags:       nil,
				Type:       "repair",
			},
			expectedErr: nil,
		},
		{
			name: "repair, sdc ref, with name override annotation",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskNameOverrideAnnotation, "override")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "override",
				Properties: map[string]any{
					"dc":                    []string{"dc1", "!otherdc*"},
					"keyspace":              []string{"keyspace", "!keyspace.table_prefix_*"},
					"fail_fast":             true,
					"host":                  "10.0.0.1",
					"ignore_down_hosts":     false,
					"intensity":             float64(1),
					"parallel":              int64(1),
					"small_table_threshold": int64(1073741824),
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "1m0s",
					StartDate:  pointer.Ptr(strfmt.DateTime(validTime)),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "repair",
			},
			expectedErr: nil,
		},
		{
			name: "repair, sdc ref, with valid intensity override annotation",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskRepairIntensityOverrideAnnotation, "0.5")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "repair",
				Properties: map[string]any{
					"dc":                    []string{"dc1", "!otherdc*"},
					"keyspace":              []string{"keyspace", "!keyspace.table_prefix_*"},
					"fail_fast":             true,
					"host":                  "10.0.0.1",
					"ignore_down_hosts":     false,
					"intensity":             float64(0.5),
					"parallel":              int64(1),
					"small_table_threshold": int64(1073741824),
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "1m0s",
					StartDate:  pointer.Ptr(strfmt.DateTime(validTime)),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "repair",
			},
			expectedErr: nil,
		},
		{
			name: "repair, sdc ref, with invalid intensity override annotation",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskRepairIntensityOverrideAnnotation, "invalid")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: nil,
			expected:        nil,
			expectedErr:     fmt.Errorf("can't make ScyllaDB Manager client repair task properties: %w", apimachineryutilerrors.NewAggregate([]error{fmt.Errorf("can't parse intensity override: %w", &strconv.NumError{Func: "ParseFloat", Num: "invalid", Err: strconv.ErrSyntax})})),
		},
		{
			name: "repair, sdc ref, with interval override annotation",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleIntervalOverrideAnnotation, "7d")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "repair",
				Properties: map[string]any{
					"dc":                    []string{"dc1", "!otherdc*"},
					"keyspace":              []string{"keyspace", "!keyspace.table_prefix_*"},
					"fail_fast":             true,
					"host":                  "10.0.0.1",
					"ignore_down_hosts":     false,
					"intensity":             float64(1),
					"parallel":              int64(1),
					"small_table_threshold": int64(1073741824),
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "7d",
					NumRetries: 3,
					RetryWait:  "1m0s",
					StartDate:  pointer.Ptr(strfmt.DateTime(validTime)),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "repair",
			},
			expectedErr: nil,
		},
		{
			name: "repair, sdc ref, with valid schedule startDate override annotation",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation, "2025-05-08T17:24:00.000Z")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "repair",
				Properties: map[string]any{
					"dc":                    []string{"dc1", "!otherdc*"},
					"keyspace":              []string{"keyspace", "!keyspace.table_prefix_*"},
					"fail_fast":             true,
					"host":                  "10.0.0.1",
					"ignore_down_hosts":     false,
					"intensity":             float64(1),
					"parallel":              int64(1),
					"small_table_threshold": int64(1073741824),
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "1m0s",
					StartDate:  pointer.Ptr(helpers.Must(strfmt.ParseDateTime("2025-05-08T17:24:00.000Z"))),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "repair",
			},
			expectedErr: nil,
		},
		{
			name: "repair, sdc ref, with invalid schedule startDate override annotation",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation, "now+invalid")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: nil,
			expected:        nil,
			expectedErr:     fmt.Errorf("can't make ScyllaDB Manager client schedule: %w", apimachineryutilerrors.NewAggregate([]error{fmt.Errorf("can't parse start date: %w", fmt.Errorf("can't parse duration: %w", errors.New("time: invalid duration +invalid")))})),
		},
		{
			name: "repair, sdc ref, with invalid schedule startDate override annotation and start date retention override option",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation, "now+invalid")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: []scyllaDBManagerClientTaskOverrideOption{
				withScheduleStartDateNowSyntaxRetention(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
			},
			expected:    nil,
			expectedErr: fmt.Errorf("can't make ScyllaDB Manager client schedule: %w", apimachineryutilerrors.NewAggregate([]error{fmt.Errorf("can't parse start date: %w", fmt.Errorf("can't parse duration: %w", errors.New("time: invalid duration +invalid")))})),
		},
		{
			name: "repair, sdc ref, with 'now' schedule startDate override annotation and start date retention override option",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation, "now")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: []scyllaDBManagerClientTaskOverrideOption{
				withScheduleStartDateNowSyntaxRetention(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
			},
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "repair",
				Properties: map[string]any{
					"dc":                    []string{"dc1", "!otherdc*"},
					"keyspace":              []string{"keyspace", "!keyspace.table_prefix_*"},
					"fail_fast":             true,
					"host":                  "10.0.0.1",
					"ignore_down_hosts":     false,
					"intensity":             float64(1),
					"parallel":              int64(1),
					"small_table_threshold": int64(1073741824),
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "1m0s",
					StartDate:  pointer.Ptr(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "repair",
			},
			expectedErr: nil,
		},
		{
			name: "repair, sdc ref, with 'now+duration' schedule startDate override annotation and start date retention override option",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation, "now+3d2h10m")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: []scyllaDBManagerClientTaskOverrideOption{
				withScheduleStartDateNowSyntaxRetention(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
			},
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "repair",
				Properties: map[string]any{
					"dc":                    []string{"dc1", "!otherdc*"},
					"keyspace":              []string{"keyspace", "!keyspace.table_prefix_*"},
					"fail_fast":             true,
					"host":                  "10.0.0.1",
					"ignore_down_hosts":     false,
					"intensity":             float64(1),
					"parallel":              int64(1),
					"small_table_threshold": int64(1073741824),
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "1m0s",
					StartDate:  pointer.Ptr(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "repair",
			},
			expectedErr: nil,
		},
		{
			name: "repair, sdc ref, with valid time schedule startDate override annotation and start date retention override option",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleStartDateOverrideAnnotation, "2025-05-08T17:24:00.000Z")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: []scyllaDBManagerClientTaskOverrideOption{
				withScheduleStartDateNowSyntaxRetention(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
			},
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "repair",
				Properties: map[string]any{
					"dc":                    []string{"dc1", "!otherdc*"},
					"keyspace":              []string{"keyspace", "!keyspace.table_prefix_*"},
					"fail_fast":             true,
					"host":                  "10.0.0.1",
					"ignore_down_hosts":     false,
					"intensity":             float64(1),
					"parallel":              int64(1),
					"small_table_threshold": int64(1073741824),
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "1m0s",
					StartDate:  pointer.Ptr(helpers.Must(strfmt.ParseDateTime("2025-05-08T17:24:00.000Z"))),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "repair",
			},
			expectedErr: nil,
		},
		{
			name:            "repair, sdc ref, without startDate override annotation and with start date retention override option",
			smt:             newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: []scyllaDBManagerClientTaskOverrideOption{
				withScheduleStartDateNowSyntaxRetention(helpers.Must(strfmt.ParseDateTime("2021-01-01T11:11:11Z"))),
			},
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "repair",
				Properties: map[string]any{
					"dc":                    []string{"dc1", "!otherdc*"},
					"keyspace":              []string{"keyspace", "!keyspace.table_prefix_*"},
					"fail_fast":             true,
					"host":                  "10.0.0.1",
					"ignore_down_hosts":     false,
					"intensity":             float64(1),
					"parallel":              int64(1),
					"small_table_threshold": int64(1073741824),
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "1m0s",
					StartDate:  pointer.Ptr(strfmt.DateTime(validTime)),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "repair",
			},
			expectedErr: nil,
		},
		{
			name: "repair, sdc ref, with schedule timezone override annotation",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskScheduleTimezoneOverrideAnnotation, "CET")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "repair",
				Properties: map[string]any{
					"dc":                    []string{"dc1", "!otherdc*"},
					"keyspace":              []string{"keyspace", "!keyspace.table_prefix_*"},
					"fail_fast":             true,
					"host":                  "10.0.0.1",
					"ignore_down_hosts":     false,
					"intensity":             float64(1),
					"parallel":              int64(1),
					"small_table_threshold": int64(1073741824),
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "1m0s",
					StartDate:  pointer.Ptr(strfmt.DateTime(validTime)),
					Timezone:   "CET",
					Window:     nil,
				},
				Tags: nil,
				Type: "repair",
			},
			expectedErr: nil,
		},
		{
			name: "repair, sdc ref, with valid small table threshold override annotation",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskRepairSmallTableThresholdOverrideAnnotation, "512MiB")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: nil,
			expected: &managerclient.Task{
				ClusterID: "cluster-id",
				Enabled:   true,
				ID:        "",
				Labels: map[string]string{
					"scylla-operator.scylladb.com/managed-hash": mockManagedHash,
					"scylla-operator.scylladb.com/owner-uid":    "uid",
				},
				Name: "repair",
				Properties: map[string]any{
					"dc":                    []string{"dc1", "!otherdc*"},
					"keyspace":              []string{"keyspace", "!keyspace.table_prefix_*"},
					"fail_fast":             true,
					"host":                  "10.0.0.1",
					"ignore_down_hosts":     false,
					"intensity":             float64(1),
					"parallel":              int64(1),
					"small_table_threshold": int64(536870912),
				},
				Schedule: &managerclient.Schedule{
					Cron:       "0 23 * * SAT",
					Interval:   "",
					NumRetries: 3,
					RetryWait:  "1m0s",
					StartDate:  pointer.Ptr(strfmt.DateTime(validTime)),
					Timezone:   "",
					Window:     nil,
				},
				Tags: nil,
				Type: "repair",
			},
			expectedErr: nil,
		},
		{
			name: "repair, sdc ref, with invalid small table threshold override annotation",
			smt: func() *scyllav1alpha1.ScyllaDBManagerTask {
				smt := newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef()

				metav1.SetMetaDataAnnotation(&smt.ObjectMeta, naming.ScyllaDBManagerTaskRepairSmallTableThresholdOverrideAnnotation, "invalid size")

				return smt
			}(),
			clusterID:       "cluster-id",
			managedHashFunc: getMockManagedHash,
			overrideOptions: nil,
			expected:        nil,
			expectedErr:     fmt.Errorf("can't make ScyllaDB Manager client repair task properties: %w", apimachineryutilerrors.NewAggregate([]error{fmt.Errorf("can't parse small table threshold override: %w", fmt.Errorf("invalid byte size string %q, it must be real number with unit suffix %q", "invalid size", "B,KiB,MiB,GiB,TiB,PiB,EiB"))})),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := makeScyllaDBManagerClientTaskWithManagedHashFunc(tc.smt, tc.clusterID, tc.managedHashFunc, tc.overrideOptions...)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected and got errors differ:\n%s\n", cmp.Diff(tc.expectedErr, err, cmpopts.EquateErrors()))
			}

			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("expected and got ScyllaDB Manager client tasks differ:\n%s\n", cmp.Diff(tc.expected, got))
			}
		})
	}
}

func newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef() *scyllav1alpha1.ScyllaDBManagerTask {
	return &scyllav1alpha1.ScyllaDBManagerTask{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backup",
			Namespace: "scylla",
			UID:       "uid",
		},
		Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
			ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
				Kind: scyllav1alpha1.ScyllaDBDatacenterGVK.Kind,
				Name: "basic",
			},
			Type: scyllav1alpha1.ScyllaDBManagerTaskTypeBackup,
			Backup: &scyllav1alpha1.ScyllaDBManagerBackupTaskOptions{
				ScyllaDBManagerTaskSchedule: scyllav1alpha1.ScyllaDBManagerTaskSchedule{
					Cron:       pointer.Ptr("0 23 * * SAT"),
					NumRetries: pointer.Ptr[int64](3),
					RetryWait: &metav1.Duration{
						Duration: 1 * time.Minute,
					},
					StartDate: pointer.Ptr(metav1.NewTime(validTime)),
				},
				DC:       []string{"dc1", "!otherdc*"},
				Keyspace: []string{"keyspace", "!keyspace.table_prefix_*"},
				Location: []string{"gcs:test"},
				RateLimit: []string{
					"dc1:1",
					"2",
				},
				Retention: pointer.Ptr[int64](3),
				SnapshotParallel: []string{
					"dc1:2",
					"3",
				},
				UploadParallel: []string{
					"dc1:3",
					"4",
				},
			},
		},
	}
}

func newRepairScyllaDBManagerTaskWithScyllaDBDatacenterRef() *scyllav1alpha1.ScyllaDBManagerTask {
	return &scyllav1alpha1.ScyllaDBManagerTask{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "repair",
			Namespace: "scylla",
			UID:       "uid",
		},
		Spec: scyllav1alpha1.ScyllaDBManagerTaskSpec{
			ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{
				Kind: scyllav1alpha1.ScyllaDBDatacenterGVK.Kind,
				Name: "basic",
			},
			Type: scyllav1alpha1.ScyllaDBManagerTaskTypeRepair,
			Repair: &scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{
				ScyllaDBManagerTaskSchedule: scyllav1alpha1.ScyllaDBManagerTaskSchedule{
					Cron:       pointer.Ptr("0 23 * * SAT"),
					NumRetries: pointer.Ptr[int64](3),
					RetryWait: &metav1.Duration{
						Duration: 1 * time.Minute,
					},
					StartDate: pointer.Ptr(metav1.NewTime(validTime)),
				},
				DC:                  []string{"dc1", "!otherdc*"},
				Keyspace:            []string{"keyspace", "!keyspace.table_prefix_*"},
				FailFast:            pointer.Ptr(true),
				Host:                pointer.Ptr("10.0.0.1"),
				IgnoreDownHosts:     pointer.Ptr(false),
				Intensity:           pointer.Ptr("1"),
				Parallel:            pointer.Ptr[int64](1),
				SmallTableThreshold: pointer.Ptr(resource.MustParse("1Gi")),
			},
		},
	}
}

func Test_parseByteCount(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name        string
		byteSize    string
		expected    int64
		expectedErr error
	}{
		{
			name:        "B unit",
			byteSize:    "1024B",
			expected:    1024,
			expectedErr: nil,
		},
		{
			name:        "KiB unit",
			byteSize:    "1KiB",
			expected:    1024,
			expectedErr: nil,
		},
		{
			name:        "MiB unit",
			byteSize:    "2.5MiB",
			expected:    2621440,
			expectedErr: nil,
		},
		{
			name:        "GiB unit",
			byteSize:    "1GiB",
			expected:    1073741824,
			expectedErr: nil,
		},
		{
			name:        "TiB unit",
			byteSize:    "1TiB",
			expected:    1099511627776,
			expectedErr: nil,
		},
		{
			name:        "invalid format - no unit",
			byteSize:    "1024",
			expected:    0,
			expectedErr: fmt.Errorf("invalid byte size string %q, it must be real number with unit suffix %q", "1024", "B,KiB,MiB,GiB,TiB,PiB,EiB"),
		},
		{
			name:        "invalid format - invalid unit",
			byteSize:    "1024KB",
			expected:    0,
			expectedErr: fmt.Errorf("invalid byte size string %q, it must be real number with unit suffix %q", "1024KB", "B,KiB,MiB,GiB,TiB,PiB,EiB"),
		},
		{
			name:        "invalid format - invalid number",
			byteSize:    "abc GiB",
			expected:    0,
			expectedErr: fmt.Errorf("invalid byte size string %q, it must be real number with unit suffix %q", "abc GiB", "B,KiB,MiB,GiB,TiB,PiB,EiB"),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, err := parseByteCount(tc.byteSize)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected and got errors differ:\n%s\n", cmp.Diff(tc.expectedErr, err))
			}

			if result != tc.expected {
				t.Errorf("expected %d, got %d", tc.expected, result)
			}
		})
	}
}

func Test_makeScyllaDBManagerClientRepairTaskPropertiesFractionalIntensity(t *testing.T) {
	t.Parallel()

	properties, err := makeScyllaDBManagerClientRepairTaskProperties(&scyllav1alpha1.ScyllaDBManagerRepairTaskOptions{
		Intensity: pointer.Ptr("0.25"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if diff := cmp.Diff(map[string]any{"intensity": float64(0.25)}, properties); diff != "" {
		t.Fatalf("expected and got repair properties differ:\n%s", diff)
	}
}

func Test_parseStartDate(t *testing.T) {
	t.Parallel()

	mockNowFunc := func() time.Time {
		return time.Date(2025, 5, 29, 13, 26, 5, 0, time.UTC)
	}

	tt := []struct {
		name        string
		startDate   string
		nowFunc     func() time.Time
		expected    strfmt.DateTime
		expectedErr error
	}{
		{
			name:        "exact 'now'",
			startDate:   "now",
			nowFunc:     mockNowFunc,
			expected:    strfmt.DateTime{},
			expectedErr: nil,
		},
		{
			name:        "now plus 1 hour",
			startDate:   "now+1h",
			nowFunc:     mockNowFunc,
			expected:    strfmt.DateTime(time.Date(2025, 5, 29, 14, 26, 5, 0, time.UTC)),
			expectedErr: nil,
		},
		{
			name:        "now plus complex duration",
			startDate:   "now+3d2h10m",
			nowFunc:     mockNowFunc,
			expected:    strfmt.DateTime(time.Date(2025, 6, 01, 15, 36, 5, 0, time.UTC)),
			expectedErr: nil,
		},
		{
			name:        "now plus zero duration",
			startDate:   "now+0h",
			nowFunc:     mockNowFunc,
			expected:    strfmt.DateTime{},
			expectedErr: nil,
		},
		{
			name:        "now plus invalid duration",
			startDate:   "now+invalid",
			nowFunc:     mockNowFunc,
			expected:    strfmt.DateTime{},
			expectedErr: fmt.Errorf("can't parse duration: %w", errors.New("time: invalid duration +invalid")),
		},
		{
			name:        "RFC3339 timestamp",
			startDate:   "2023-05-08T17:24:00Z",
			nowFunc:     mockNowFunc,
			expected:    strfmt.DateTime(time.Date(2023, 5, 8, 17, 24, 0, 0, time.UTC)),
			expectedErr: nil,
		},
		{
			name:        "RFC3339 timestamp with milliseconds",
			startDate:   "2023-05-08T17:24:00.123Z",
			nowFunc:     mockNowFunc,
			expected:    strfmt.DateTime(time.Date(2023, 5, 8, 17, 24, 0, 123000000, time.UTC)),
			expectedErr: nil,
		},
		{
			name:      "invalid timestamp format",
			startDate: "2023-05-08 17:24:00",
			nowFunc:   mockNowFunc,
			expected:  strfmt.DateTime{},
			expectedErr: fmt.Errorf("can't parse time: %w", &time.ParseError{
				Layout:     time.RFC3339,
				Value:      "2023-05-08 17:24:00",
				LayoutElem: "T",
				ValueElem:  " 17:24:00",
				Message:    "",
			}),
		},
		{
			name:      "empty string",
			startDate: "",
			nowFunc:   mockNowFunc,
			expected:  strfmt.DateTime{},
			expectedErr: fmt.Errorf("can't parse time: %w", &time.ParseError{
				Layout:     time.RFC3339,
				Value:      "",
				LayoutElem: "2006",
				ValueElem:  "",
				Message:    "",
			}),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseStartDate(tc.startDate, tc.nowFunc)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected and got errors differ:\n%s\n", cmp.Diff(tc.expectedErr, err, cmpopts.EquateErrors()))
			}

			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("expected and got differ:\n%s\n", cmp.Diff(tc.expected, got))
			}
		})
	}
}
