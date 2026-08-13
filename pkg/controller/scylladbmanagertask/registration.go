// Copyright (C) 2026 ScyllaDB

package scylladbmanagertask

import (
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"k8s.io/apimachinery/pkg/labels"
)

// scyllaDBManagerClusterRegistrationForTask resolves registration ownership by
// the typed cluster reference, not by a generated object name. Secure external
// Manager registrations are intentionally allowed to have explicit names.
// Requiring exactly one match makes duplicate authorities fail closed.
func (smtc *Controller) scyllaDBManagerClusterRegistrationForTask(smt *scyllav1alpha1.ScyllaDBManagerTask) (*scyllav1alpha1.ScyllaDBManagerClusterRegistration, string, error) {
	expectedName, err := naming.ScyllaDBManagerClusterRegistrationNameForScyllaDBManagerTask(smt)
	if err != nil {
		return nil, "", fmt.Errorf("can't get conventional ScyllaDBManagerClusterRegistration name: %w", err)
	}

	registrations, err := smtc.scyllaDBManagerClusterRegistrationLister.ScyllaDBManagerClusterRegistrations(smt.Namespace).List(labels.Everything())
	if err != nil {
		return nil, expectedName, fmt.Errorf("can't list ScyllaDBManagerClusterRegistrations: %w", err)
	}

	var match *scyllav1alpha1.ScyllaDBManagerClusterRegistration
	for _, registration := range registrations {
		if registration.Spec.ScyllaDBClusterRef != smt.Spec.ScyllaDBClusterRef {
			continue
		}
		if match != nil {
			return nil, expectedName, fmt.Errorf("multiple ScyllaDBManagerClusterRegistrations target %s/%s %q: %q and %q", smt.Spec.ScyllaDBClusterRef.Kind, smt.Namespace, smt.Spec.ScyllaDBClusterRef.Name, match.Name, registration.Name)
		}
		match = registration
	}

	return match, expectedName, nil
}
