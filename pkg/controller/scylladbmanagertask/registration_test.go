// Copyright (C) 2026 ScyllaDB

package scylladbmanagertask

import (
	"strings"
	"testing"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav1alpha1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func TestScyllaDBManagerClusterRegistrationForTask(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		registrationNames []string
		expectedName      string
		expectedErr       string
	}{
		{
			name:              "explicit registration name",
			registrationNames: []string{"scylladb"},
			expectedName:      "scylladb",
		},
		{
			name: "registration absent",
		},
		{
			name:              "duplicate authority fails closed",
			registrationNames: []string{"scylladb", "duplicate"},
			expectedErr:       "multiple ScyllaDBManagerClusterRegistrations target",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			smt := newBackupScyllaDBManagerTaskWithScyllaDBDatacenterRef()
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			for _, name := range tc.registrationNames {
				if err := indexer.Add(&scyllav1alpha1.ScyllaDBManagerClusterRegistration{
					ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: smt.Namespace},
					Spec: scyllav1alpha1.ScyllaDBManagerClusterRegistrationSpec{
						ScyllaDBClusterRef: smt.Spec.ScyllaDBClusterRef,
					},
				}); err != nil {
					t.Fatalf("add registration: %v", err)
				}
			}

			controller := &Controller{
				scyllaDBManagerClusterRegistrationLister: scyllav1alpha1listers.NewScyllaDBManagerClusterRegistrationLister(indexer),
			}
			registration, conventionalName, err := controller.scyllaDBManagerClusterRegistrationForTask(smt)
			if tc.expectedErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.expectedErr) {
					t.Fatalf("expected error containing %q, got %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolve registration: %v", err)
			}
			if conventionalName == "" {
				t.Fatal("expected conventional name for diagnostics")
			}
			if tc.expectedName == "" {
				if registration != nil {
					t.Fatalf("expected no registration, got %q", registration.Name)
				}
				return
			}
			if registration == nil || registration.Name != tc.expectedName {
				t.Fatalf("expected registration %q, got %#v", tc.expectedName, registration)
			}
		})
	}
}
