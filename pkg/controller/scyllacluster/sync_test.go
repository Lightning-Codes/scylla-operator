// Copyright (c) 2024 ScyllaDB.

package scyllacluster

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/internalapi"
	"github.com/scylladb/scylla-operator/pkg/naming"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStableScyllaClusterUsesDirectManagerRegistration(t *testing.T) {
	t.Parallel()

	sc := &scyllav1.ScyllaCluster{ObjectMeta: metav1.ObjectMeta{Name: "scylladb", Namespace: "sophena"}}
	registrationName, err := naming.ScyllaDBManagerClusterRegistrationNameForScyllaCluster(sc)
	if err != nil {
		t.Fatalf("can't calculate direct stable registration name: %v", err)
	}
	clusterID := "manager-cluster-id"
	directRegistration := &scyllav1alpha1.ScyllaDBManagerClusterRegistration{
		ObjectMeta: metav1.ObjectMeta{Name: registrationName, Namespace: sc.Namespace},
		Spec: scyllav1alpha1.ScyllaDBManagerClusterRegistrationSpec{
			ScyllaDBClusterRef: scyllav1alpha1.LocalScyllaDBReference{Kind: scyllav1.ScyllaClusterGVK.Kind, Name: sc.Name},
		},
		Status: scyllav1alpha1.ScyllaDBManagerClusterRegistrationStatus{ClusterID: &clusterID},
	}

	filtered, err := filterScyllaDBManagerClusterRegistrations(sc, []*scyllav1alpha1.ScyllaDBManagerClusterRegistration{directRegistration})
	if err != nil {
		t.Fatalf("can't filter direct stable registration: %v", err)
	}
	if diff := cmp.Diff([]*scyllav1alpha1.ScyllaDBManagerClusterRegistration{directRegistration}, filtered); diff != "" {
		t.Fatalf("stable registration filter differs (-want +got):\n%s", diff)
	}
	found, ok, err := getScyllaDBManagerClusterRegistration(sc, filtered)
	if err != nil {
		t.Fatalf("can't resolve direct stable registration status: %v", err)
	}
	if !ok || found != directRegistration || found.Spec.ScyllaDBClusterRef.Kind != scyllav1.ScyllaClusterGVK.Kind {
		t.Fatalf("stable registration wasn't resolved directly: found=%t registration=%#v", ok, found)
	}
}

func TestOffsetSDCConditionsToSCGenerationSpace(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name               string
		scGeneration       int64
		sdcGeneration      int64
		conditions         []metav1.Condition
		expectedConditions []metav1.Condition
	}{
		{
			name:               "nil conditions returns nil",
			scGeneration:       5,
			sdcGeneration:      3,
			conditions:         nil,
			expectedConditions: nil,
		},
		{
			name:               "empty conditions returns empty slice",
			scGeneration:       5,
			sdcGeneration:      3,
			conditions:         []metav1.Condition{},
			expectedConditions: []metav1.Condition{},
		},
		{
			name:          "equal generations leaves ObservedGenerations unchanged",
			scGeneration:  5,
			sdcGeneration: 5,
			conditions: []metav1.Condition{
				{Type: "Progressing", ObservedGeneration: 5},
				{Type: "Degraded", ObservedGeneration: 5},
			},
			expectedConditions: []metav1.Condition{
				{Type: "Progressing", ObservedGeneration: 5},
				{Type: "Degraded", ObservedGeneration: 5},
			},
		},
		{
			name:          "SC.Generation > SDC.Generation shifts ObservedGenerations up",
			scGeneration:  5,
			sdcGeneration: 3,
			conditions: []metav1.Condition{
				{Type: "Progressing", ObservedGeneration: 3},
				{Type: "Degraded", ObservedGeneration: 3},
			},
			expectedConditions: []metav1.Condition{
				{Type: "Progressing", ObservedGeneration: 5},
				{Type: "Degraded", ObservedGeneration: 5},
			},
		},
		{
			name:          "SDC.Generation > SC.Generation shifts ObservedGenerations down (without clamping)",
			scGeneration:  3,
			sdcGeneration: 5,
			conditions: []metav1.Condition{
				{Type: "Progressing", ObservedGeneration: 5},
				{Type: "Degraded", ObservedGeneration: 5},
			},
			expectedConditions: []metav1.Condition{
				{Type: "Progressing", ObservedGeneration: 3},
				{Type: "Degraded", ObservedGeneration: 3},
			},
		},
		{
			name:          "SDC.Generation > SC.Generation shifts ObservedGenerations down (with clamping)",
			scGeneration:  1,
			sdcGeneration: 5,
			conditions: []metav1.Condition{
				{Type: "Progressing", ObservedGeneration: 3},
				{Type: "Degraded", ObservedGeneration: 3},
			},
			expectedConditions: []metav1.Condition{
				{Type: "Progressing", ObservedGeneration: 0}, // 3 + (1-5) = -1 → 0
				{Type: "Degraded", ObservedGeneration: 0},    // 3 + (1-5) = -1 → 0
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			originalConditions := slices.Clone(tc.conditions)
			got := offsetSDCConditionsToSCGenerationSpace(tc.scGeneration, tc.sdcGeneration, tc.conditions)

			if !apiequality.Semantic.DeepEqual(got, tc.expectedConditions) {
				t.Errorf("expected and got conditions differ:\n%s", cmp.Diff(tc.expectedConditions, got))
			}

			// Verify the input was not mutated.
			if !apiequality.Semantic.DeepEqual(tc.conditions, originalConditions) {
				t.Errorf("input conditions were mutated:\n%s", cmp.Diff(originalConditions, tc.conditions))
			}
		})
	}
}

func TestFindStatusConditionOrUnknown(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name              string
		conditions        []metav1.Condition
		conditionType     string
		generation        int64
		expectedCondition metav1.Condition
	}{
		{
			name:          "nil conditions returns unknown baseline",
			conditions:    nil,
			conditionType: "Available",
			generation:    3,
			expectedCondition: metav1.Condition{
				Type:               "Available",
				Status:             metav1.ConditionUnknown,
				ObservedGeneration: 3,
				Reason:             internalapi.AwaitingConditionReason,
				Message:            "",
			},
		},
		{
			name:          "empty conditions returns unknown baseline",
			conditions:    []metav1.Condition{},
			conditionType: "Available",
			generation:    3,
			expectedCondition: metav1.Condition{
				Type:               "Available",
				Status:             metav1.ConditionUnknown,
				ObservedGeneration: 3,
				Reason:             internalapi.AwaitingConditionReason,
				Message:            "",
			},
		},
		{
			name: "matching type and generation returns the condition",
			conditions: []metav1.Condition{
				{
					Type:               "Available",
					Status:             metav1.ConditionTrue,
					ObservedGeneration: 3,
					Reason:             "AsExpected",
					Message:            "",
				},
			},
			conditionType: "Available",
			generation:    3,
			expectedCondition: metav1.Condition{
				Type:               "Available",
				Status:             metav1.ConditionTrue,
				ObservedGeneration: 3,
				Reason:             "AsExpected",
				Message:            "",
			},
		},
		{
			name: "matching type but wrong ObservedGeneration returns unknown baseline",
			conditions: []metav1.Condition{
				{
					Type:               "Available",
					Status:             metav1.ConditionTrue,
					ObservedGeneration: 2,
					Reason:             "AsExpected",
					Message:            "",
				},
			},
			conditionType: "Available",
			generation:    3,
			expectedCondition: metav1.Condition{
				Type:               "Available",
				Status:             metav1.ConditionUnknown,
				ObservedGeneration: 3,
				Reason:             internalapi.AwaitingConditionReason,
				Message:            "",
			},
		},
		{
			name: "no matching type returns unknown baseline",
			conditions: []metav1.Condition{
				{
					Type:               "Progressing",
					Status:             metav1.ConditionFalse,
					ObservedGeneration: 3,
					Reason:             "AsExpected",
					Message:            "",
				},
			},
			conditionType: "Available",
			generation:    3,
			expectedCondition: metav1.Condition{
				Type:               "Available",
				Status:             metav1.ConditionUnknown,
				ObservedGeneration: 3,
				Reason:             internalapi.AwaitingConditionReason,
				Message:            "",
			},
		},
		{
			name: "multiple conditions, correct one is returned",
			conditions: []metav1.Condition{
				{
					Type:               "Progressing",
					Status:             metav1.ConditionFalse,
					ObservedGeneration: 3,
					Reason:             "AsExpected",
					Message:            "",
				},
				{
					Type:               "Available",
					Status:             metav1.ConditionTrue,
					ObservedGeneration: 3,
					Reason:             "AsExpected",
					Message:            "",
				},
				{
					Type:               "Degraded",
					Status:             metav1.ConditionFalse,
					ObservedGeneration: 3,
					Reason:             "AsExpected",
					Message:            "",
				},
			},
			conditionType: "Available",
			generation:    3,
			expectedCondition: metav1.Condition{
				Type:               "Available",
				Status:             metav1.ConditionTrue,
				ObservedGeneration: 3,
				Reason:             "AsExpected",
				Message:            "",
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := findStatusConditionOrUnknown(tc.conditions, tc.conditionType, tc.generation)
			if !apiequality.Semantic.DeepEqual(got, tc.expectedCondition) {
				t.Errorf("expected and got conditions differ:\n%s", cmp.Diff(tc.expectedCondition, got))
			}
		})
	}
}
