// Copyright (C) 2025 ScyllaDB

package scylladbmanagerclusterregistration

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/naming"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

func (smcrc *Controller) syncFinalizer(ctx context.Context, smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition
	var err error

	if !smcrc.hasFinalizer(smcr.GetFinalizers()) {
		klog.V(4).InfoS("Object is already finalized", "ScyllaDBManagerClusterRegistration", klog.KObj(smcr), "UID", smcr.UID)
		return progressingConditions, nil
	}

	klog.V(4).InfoS("Finalizing object", "ScyllaDBManagerClusterRegistration", klog.KObj(smcr), "UID", smcr.UID)

	// Finalization is deliberately fail-closed. Namespace deletion is not proof that
	// Manager state disappeared, so even registrations carrying the legacy global label
	// must use their configured verified mTLS client before the finalizer is removed.
	managerClient, err := controllerhelpers.GetSecureScyllaDBManagerClient(ctx, smcrc.kubeClient, smcr)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't get manager client: %w", err)
	}

	managerCluster, found, err := getScyllaDBManagerCluster(ctx, smcr, managerClient)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't get ScyllaDB Manager cluster: %w", err)
	}

	if found {
		err = managerClient.DeleteCluster(ctx, managerCluster.ID)
		if err != nil {
			klog.V(4).InfoS("Failed to delete ScyllaDB Manager cluster", "ScyllaDBManagerClusterRegistration", klog.KObj(smcr), "UID", smcr.UID, "ScyllaDBManagerClusterName", managerCluster.Name, "ScyllaDBManagerClusterID", managerCluster.ID, "Error", err)
			return progressingConditions, fmt.Errorf("can't delete ScyllaDB Manager cluster: %w", err)
		}

		klog.V(4).InfoS("Deleted the ScyllaDB Manager cluster.", "ScyllaDBManagerClusterRegistration", klog.KObj(smcr), "UID", smcr.UID, "ScyllaDBManagerClusterName", managerCluster.Name, "ScyllaDBManagerClusterID", managerCluster.ID)
		progressingConditions = append(progressingConditions, metav1.Condition{
			Type:               scyllaDBManagerClusterRegistrationFinalizerProgressingCondition,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: smcr.Generation,
			Reason:             "DeletedScyllaDBManagerCluster",
			Message:            "Deleted the cluster from ScyllaDB Manager state.",
		})

		return progressingConditions, nil
	}

	klog.V(4).InfoS("ScyllaDB Manager cluster has already been deleted. Removing finalizer.", "ScyllaDBManagerClusterRegistration", klog.KObj(smcr), "UID", smcr.UID)
	err = smcrc.removeFinalizer(ctx, smcr)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't remove finalizer: %w", err)
	}

	return progressingConditions, nil
}

func (smcrc *Controller) removeFinalizer(ctx context.Context, smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration) error {
	patch, err := controllerhelpers.RemoveFinalizerPatch(smcr, naming.ScyllaDBManagerClusterRegistrationFinalizer)
	if err != nil {
		return fmt.Errorf("can't create remove finalizer patch: %w", err)
	}

	_, err = smcrc.scyllaClient.ScyllaV1alpha1().ScyllaDBManagerClusterRegistrations(smcr.Namespace).Patch(ctx, smcr.Name, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("can't patch ScyllaDBManagerClusterRegistration %q: %w", naming.ObjRef(smcr), err)
	}

	klog.V(2).InfoS("Removed finalizer from ScyllaDBManagerClusterRegistration", "ScyllaDBManagerClusterRegistration", klog.KObj(smcr))
	return nil
}
