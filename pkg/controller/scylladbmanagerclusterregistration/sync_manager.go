// Copyright (C) 2025 ScyllaDB

package scylladbmanagerclusterregistration

import (
	"context"
	"fmt"
	"sort"

	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/pkg/managerclientsecure"
	"github.com/scylladb/scylla-operator/pkg/naming"
	hashutil "github.com/scylladb/scylla-operator/pkg/util/hash"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

type resolvedScyllaDBManagerClusterConnection struct {
	AuthToken                 string
	Username                  string
	Password                  string
	AlternatorAccessKeyID     string
	AlternatorSecretAccessKey string
	CQLCA                     []byte
	CQLClientCertificate      []byte
	CQLClientPrivateKey       []byte
	AlternatorCA              []byte
	AgentCA                   []byte
	Revision                  string
}

type referencedObjectRevision struct {
	Kind            string
	Namespace       string
	Name            string
	Key             string
	ResourceVersion string
}

type nonSecretConnectionRevision struct {
	Host                 string
	ManagerAPIURL        string
	ManagerAPIServerName string
	CQLServerName        string
	AlternatorServerName string
	AgentServerName      string
	ReferencedObjects    []referencedObjectRevision
}

func (smcrc *Controller) syncManager(
	ctx context.Context,
	smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration,
	status *scyllav1alpha1.ScyllaDBManagerClusterRegistrationStatus,
) ([]metav1.Condition, error) {
	var progressingConditions []metav1.Condition

	host, authTokenSecretName, progressingCondition, err := smcrc.registrationConnectionTarget(smcr)
	if err != nil {
		setConnectionVerificationUnavailable(status, smcr, "ScyllaDBConnectionTargetResolutionFailed", "The ScyllaDB connection target could not be resolved.")
		return progressingConditions, err
	}
	if progressingCondition != nil {
		setConnectionVerificationUnavailable(status, smcr, "AwaitingScyllaDBConnectionTarget", "Awaiting the referenced ScyllaDB cluster to become available.")
		progressingConditions = append(progressingConditions, *progressingCondition)
		return progressingConditions, nil
	}

	connection, err := smcrc.resolveRegistrationConnection(smcr, host, authTokenSecretName)
	if err != nil {
		status.ConnectionRevision = nil
		setConnectionVerificationUnavailable(status, smcr, "ConnectionMaterialResolutionFailed", "Required connection material could not be resolved.")
		return progressingConditions, fmt.Errorf("can't resolve connection material: %w", err)
	}
	status.ConnectionRevision = &connection.Revision

	requiredManagerCluster := makeRequiredScyllaDBManagerCluster(
		scyllaDBManagerClusterName(smcr),
		string(smcr.UID),
		host,
		smcr,
		connection,
	)

	managerClient, err := controllerhelpers.GetSecureScyllaDBManagerClient(ctx, smcrc.kubeClient, smcr)
	if err != nil {
		setConnectionVerificationUnavailable(status, smcr, "ManagerAPITLSConfigurationFailed", "The verified Manager API client could not be configured.")
		return progressingConditions, fmt.Errorf("can't get verified Manager API client: %w", err)
	}

	managerCluster, found, err := getScyllaDBManagerCluster(ctx, smcr, managerClient)
	if err != nil {
		setConnectionVerificationUnavailable(status, smcr, "ManagerAPIRequestFailed", "The verified Manager API request failed.")
		return progressingConditions, fmt.Errorf("can't get ScyllaDB Manager cluster: %w", err)
	}
	setManagerAPIConnectionVerified(status, smcr, metav1.ConditionTrue, "VerifiedManagerAPIConnection", "Connected to ScyllaDB Manager using verified mutual TLS.")

	if !found {
		setDatabaseConnectionVerified(status, smcr, metav1.ConditionFalse, "AwaitingDatabaseConnectionVerification", "Awaiting Manager verification of all configured database connections.")
		klog.V(4).InfoS("Creating ScyllaDB Manager cluster.", "ScyllaDBManagerClusterRegistration", klog.KObj(smcr), "ScyllaDBManagerClusterName", requiredManagerCluster.Name)
		managerClusterID, err := managerClient.CreateCluster(ctx, requiredManagerCluster)
		if err != nil {
			setManagerAPIConnectionVerified(status, smcr, metav1.ConditionFalse, "ManagerAPIRequestFailed", "The verified Manager API request failed.")
			return progressingConditions, fmt.Errorf("can't create ScyllaDB Manager cluster %q: %w", requiredManagerCluster.Name, err)
		}

		status.ClusterID = &managerClusterID
		progressingConditions = append(progressingConditions, metav1.Condition{
			Type:               managerControllerProgressingCondition,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: smcr.Generation,
			Reason:             "CreatedScyllaDBManagerCluster",
			Message:            fmt.Sprintf("Created a ScyllaDB Manager cluster: %s (%s).", requiredManagerCluster.Name, managerClusterID),
		})
		return progressingConditions, nil
	}

	ownerUIDLabelValue, hasOwnerUIDLabel := managerCluster.Labels[naming.OwnerUIDLabel]
	if !hasOwnerUIDLabel {
		setDatabaseConnectionVerified(status, smcr, metav1.ConditionFalse, "AwaitingDatabaseConnectionVerification", "Awaiting Manager verification of all configured database connections.")
		klog.Warningf("ScyllaDB Manager cluster %q is missing the owner UID label. Deleting it to avoid a name collision.", managerCluster.Name)
		if err := managerClient.DeleteCluster(ctx, managerCluster.ID); err != nil {
			setManagerAPIConnectionVerified(status, smcr, metav1.ConditionFalse, "ManagerAPIRequestFailed", "The verified Manager API request failed.")
			return progressingConditions, fmt.Errorf("can't delete ScyllaDB Manager cluster %q: %w", managerCluster.Name, err)
		}

		progressingConditions = append(progressingConditions, metav1.Condition{
			Type:               managerControllerProgressingCondition,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: smcr.Generation,
			Reason:             "DeletedCollidingScyllaDBManagerCluster",
			Message:            "Deleted a colliding ScyllaDB Manager cluster with no OwnerUID label.",
		})
		return progressingConditions, nil
	}

	status.ClusterID = &managerCluster.ID
	if ownerUIDLabelValue != string(smcr.UID) {
		klog.Warningf("Cluster %q already exists in ScyllaDB Manager state and has an owner UID label (%q), but it has a different owner. ScyllaDBManagerClusterRegistration %q will adopt it.", managerCluster.Name, ownerUIDLabelValue, klog.KObj(smcr))
	}

	if ownerUIDLabelValue != string(smcr.UID) || requiredManagerCluster.Labels[naming.ManagedHash] != managerCluster.Labels[naming.ManagedHash] {
		setDatabaseConnectionVerified(status, smcr, metav1.ConditionFalse, "AwaitingDatabaseConnectionVerification", "Awaiting Manager verification of the updated database connection.")
		requiredManagerCluster.ID = managerCluster.ID
		klog.V(4).InfoS("Updating ScyllaDB Manager cluster.", "ScyllaDBManagerClusterRegistration", klog.KObj(smcr), "ScyllaDBManagerClusterName", requiredManagerCluster.Name, "ScyllaDBManagerClusterID", requiredManagerCluster.ID)
		if err := managerClient.UpdateCluster(ctx, requiredManagerCluster); err != nil {
			setManagerAPIConnectionVerified(status, smcr, metav1.ConditionFalse, "ManagerAPIRequestFailed", "The verified Manager API request failed.")
			return progressingConditions, fmt.Errorf("can't update ScyllaDB Manager cluster %q: %w", managerCluster.Name, err)
		}

		progressingConditions = append(progressingConditions, metav1.Condition{
			Type:               managerControllerProgressingCondition,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: smcr.Generation,
			Reason:             "UpdatedScyllaDBManagerCluster",
			Message:            fmt.Sprintf("Updated a ScyllaDB Manager cluster: %s (%s).", managerCluster.Name, managerCluster.ID),
		})
		return progressingConditions, nil
	}

	managerStatus, err := managerClient.ClusterStatus(ctx, managerCluster.ID)
	if err != nil {
		setConnectionVerificationUnavailable(status, smcr, "ManagerAPIRequestFailed", "The verified Manager API request failed.")
		return progressingConditions, fmt.Errorf("can't get ScyllaDB Manager cluster verification status: %w", err)
	}
	setDatabaseConnectionVerificationFromManager(status, smcr, managerStatus)
	// Health can degrade without any Kubernetes object changing. Always refresh the
	// Manager-reported verification status so a once-true condition can't indefinitely
	// authorize task reconciliation after TLS or authentication starts failing.
	smcrc.queue.AddAfter(smcr.Namespace+"/"+smcr.Name, managerVerificationPollInterval)

	return progressingConditions, nil
}

func (smcrc *Controller) registrationConnectionTarget(smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration) (string, string, *metav1.Condition, error) {
	switch smcr.Spec.ScyllaDBClusterRef.Kind {
	case scyllav1.ScyllaClusterGVK.Kind:
		sc, err := smcrc.scyllaClusterLister.ScyllaClusters(smcr.Namespace).Get(smcr.Spec.ScyllaDBClusterRef.Name)
		if err != nil {
			return "", "", nil, fmt.Errorf("can't get ScyllaCluster %q: %w", naming.ManualRef(smcr.Namespace, smcr.Spec.ScyllaDBClusterRef.Name), err)
		}
		if sc.Status.AvailableMembers == nil || *sc.Status.AvailableMembers == 0 {
			return "", "", &metav1.Condition{
				Type:               managerControllerProgressingCondition,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: smcr.Generation,
				Reason:             "AwaitingScyllaClusterAvailability",
				Message:            fmt.Sprintf("Awaiting ScyllaCluster %q availability.", naming.ObjRef(sc)),
			}, nil
		}
		return naming.CrossNamespaceServiceNameForCluster(sc), naming.AgentAuthTokenSecretNameForScyllaCluster(sc), nil, nil

	case scyllav1alpha1.ScyllaDBDatacenterGVK.Kind:
		sdc, err := smcrc.scyllaDBDatacenterLister.ScyllaDBDatacenters(smcr.Namespace).Get(smcr.Spec.ScyllaDBClusterRef.Name)
		if err != nil {
			return "", "", nil, fmt.Errorf("can't get ScyllaDBDatacenter %q: %w", naming.ManualRef(smcr.Namespace, smcr.Spec.ScyllaDBClusterRef.Name), err)
		}
		if sdc.Status.AvailableNodes == nil || *sdc.Status.AvailableNodes == 0 {
			return "", "", &metav1.Condition{
				Type:               managerControllerProgressingCondition,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: smcr.Generation,
				Reason:             "AwaitingScyllaDBDatacenterAvailability",
				Message:            fmt.Sprintf("Awaiting ScyllaDBDatacenter %q availability.", naming.ObjRef(sdc)),
			}, nil
		}
		return naming.CrossNamespaceServiceName(sdc), naming.AgentAuthTokenSecretName(sdc), nil, nil

	case scyllav1alpha1.ScyllaDBClusterGVK.Kind:
		sc, err := smcrc.scyllaDBClusterLister.ScyllaDBClusters(smcr.Namespace).Get(smcr.Spec.ScyllaDBClusterRef.Name)
		if err != nil {
			return "", "", nil, fmt.Errorf("can't get ScyllaDBCluster %q: %w", naming.ManualRef(smcr.Namespace, smcr.Spec.ScyllaDBClusterRef.Name), err)
		}
		if sc.Status.AvailableNodes == nil || *sc.Status.AvailableNodes == 0 {
			return "", "", &metav1.Condition{
				Type:               managerControllerProgressingCondition,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: smcr.Generation,
				Reason:             "AwaitingScyllaDBClusterAvailability",
				Message:            fmt.Sprintf("Awaiting ScyllaDBCluster %q availability.", naming.ObjRef(sc)),
			}, nil
		}
		host, err := naming.InterNamespaceLocalIdentityServiceAddress(sc)
		if err != nil {
			return "", "", nil, fmt.Errorf("can't get inter-namespace local identity service address for ScyllaDBCluster %q: %w", naming.ObjRef(sc), err)
		}
		authTokenSecretName, err := naming.ScyllaDBManagerAgentAuthTokenSecretNameForScyllaDBCluster(sc)
		if err != nil {
			return "", "", nil, fmt.Errorf("can't get agent auth token secret name for ScyllaDBCluster %q: %w", naming.ObjRef(sc), err)
		}
		return host, authTokenSecretName, nil, nil

	default:
		return "", "", nil, fmt.Errorf("unsupported scyllaDBClusterRef Kind: %q", smcr.Spec.ScyllaDBClusterRef.Kind)
	}
}

func (smcrc *Controller) resolveRegistrationConnection(smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration, host, authTokenSecretName string) (*resolvedScyllaDBManagerClusterConnection, error) {
	connection := &resolvedScyllaDBManagerClusterConnection{}
	revision := nonSecretConnectionRevision{
		Host:                 host,
		ManagerAPIURL:        smcr.Spec.ManagerAPI.URL,
		ManagerAPIServerName: smcr.Spec.ManagerAPI.ServerName,
	}

	authTokenSecret, err := smcrc.secretLister.Secrets(smcr.Namespace).Get(authTokenSecretName)
	if err != nil {
		return nil, fmt.Errorf("can't get Agent auth token Secret %q: %w", authTokenSecretName, err)
	}
	connection.AuthToken, err = helpers.GetAgentAuthTokenFromSecret(authTokenSecret)
	if err != nil {
		return nil, fmt.Errorf("can't get Agent auth token from Secret %q: %w", authTokenSecretName, err)
	}
	revision.ReferencedObjects = append(revision.ReferencedObjects, referencedObjectRevision{
		Kind: "Secret", Namespace: smcr.Namespace, Name: authTokenSecret.Name, Key: naming.ScyllaAgentAuthTokenFileName, ResourceVersion: authTokenSecret.ResourceVersion,
	})

	addSecretBytes := func(selector corev1.SecretKeySelector, destination *[]byte) error {
		secret, err := smcrc.secretLister.Secrets(smcr.Namespace).Get(selector.Name)
		if err != nil {
			return fmt.Errorf("can't get Secret %q: %w", selector.Name, err)
		}
		value, found := secret.Data[selector.Key]
		if !found || len(value) == 0 {
			return fmt.Errorf("Secret %q key %q is missing or empty", selector.Name, selector.Key)
		}
		*destination = append([]byte(nil), value...)
		revision.ReferencedObjects = append(revision.ReferencedObjects, referencedObjectRevision{
			Kind: "Secret", Namespace: smcr.Namespace, Name: selector.Name, Key: selector.Key, ResourceVersion: secret.ResourceVersion,
		})
		return nil
	}
	addSecretValue := func(selector corev1.SecretKeySelector, destination *string) error {
		var value []byte
		if err := addSecretBytes(selector, &value); err != nil {
			return err
		}
		*destination = string(value)
		return nil
	}
	addCAValue := func(configMapSelector *corev1.ConfigMapKeySelector, secretSelector *corev1.SecretKeySelector, destination *[]byte) error {
		if (configMapSelector == nil) == (secretSelector == nil) {
			return fmt.Errorf("exactly one CA ConfigMap or Secret selector is required")
		}

		if configMapSelector != nil {
			configMap, err := smcrc.configMapLister.ConfigMaps(smcr.Namespace).Get(configMapSelector.Name)
			if err != nil {
				return fmt.Errorf("can't get ConfigMap %q: %w", configMapSelector.Name, err)
			}
			value, found := configMap.BinaryData[configMapSelector.Key]
			if !found {
				if textValue, textFound := configMap.Data[configMapSelector.Key]; textFound {
					value, found = []byte(textValue), true
				}
			}
			if !found || len(value) == 0 {
				return fmt.Errorf("ConfigMap %q key %q is missing or empty", configMapSelector.Name, configMapSelector.Key)
			}
			*destination = append([]byte(nil), value...)
			revision.ReferencedObjects = append(revision.ReferencedObjects, referencedObjectRevision{
				Kind: "ConfigMap", Namespace: smcr.Namespace, Name: configMapSelector.Name, Key: configMapSelector.Key, ResourceVersion: configMap.ResourceVersion,
			})
			return nil
		}

		secret, err := smcrc.secretLister.Secrets(smcr.Namespace).Get(secretSelector.Name)
		if err != nil {
			return fmt.Errorf("can't get Secret %q: %w", secretSelector.Name, err)
		}
		value, found := secret.Data[secretSelector.Key]
		if !found || len(value) == 0 {
			return fmt.Errorf("Secret %q key %q is missing or empty", secretSelector.Name, secretSelector.Key)
		}
		*destination = append([]byte(nil), value...)
		revision.ReferencedObjects = append(revision.ReferencedObjects, referencedObjectRevision{
			Kind: "Secret", Namespace: smcr.Namespace, Name: secretSelector.Name, Key: secretSelector.Key, ResourceVersion: secret.ResourceVersion,
		})
		return nil
	}

	if err := addSecretValue(smcr.Spec.ManagerAPI.ClientCertificate.CertificateSecretKeyRef, new(string)); err != nil {
		return nil, fmt.Errorf("can't resolve Manager API client certificate revision: %w", err)
	}
	if err := addSecretValue(smcr.Spec.ManagerAPI.ClientCertificate.PrivateKeySecretKeyRef, new(string)); err != nil {
		return nil, fmt.Errorf("can't resolve Manager API client private key revision: %w", err)
	}
	if err := addCAValue(smcr.Spec.ManagerAPI.CAConfigMapKeyRef, smcr.Spec.ManagerAPI.CASecretKeyRef, new([]byte)); err != nil {
		return nil, fmt.Errorf("can't resolve Manager API CA revision: %w", err)
	}

	if smcr.Spec.Authentication != nil && smcr.Spec.Authentication.CQL != nil {
		if err := addSecretValue(smcr.Spec.Authentication.CQL.UsernameSecretKeyRef, &connection.Username); err != nil {
			return nil, fmt.Errorf("can't resolve CQL username: %w", err)
		}
		if err := addSecretValue(smcr.Spec.Authentication.CQL.PasswordSecretKeyRef, &connection.Password); err != nil {
			return nil, fmt.Errorf("can't resolve CQL password: %w", err)
		}
	}
	if smcr.Spec.Authentication != nil && smcr.Spec.Authentication.Alternator != nil {
		if err := addSecretValue(smcr.Spec.Authentication.Alternator.AccessKeyIDSecretKeyRef, &connection.AlternatorAccessKeyID); err != nil {
			return nil, fmt.Errorf("can't resolve Alternator access key ID: %w", err)
		}
		if err := addSecretValue(smcr.Spec.Authentication.Alternator.SecretAccessKeySecretKeyRef, &connection.AlternatorSecretAccessKey); err != nil {
			return nil, fmt.Errorf("can't resolve Alternator secret access key: %w", err)
		}
	}

	if smcr.Spec.TLS != nil && smcr.Spec.TLS.CQL != nil {
		revision.CQLServerName = smcr.Spec.TLS.CQL.ServerName
		if err := addCAValue(smcr.Spec.TLS.CQL.CAConfigMapKeyRef, smcr.Spec.TLS.CQL.CASecretKeyRef, &connection.CQLCA); err != nil {
			return nil, fmt.Errorf("can't resolve CQL CA: %w", err)
		}
		if smcr.Spec.TLS.CQL.ClientCertificate == nil {
			return nil, fmt.Errorf("CQL client certificate is required")
		}
		if err := addSecretBytes(smcr.Spec.TLS.CQL.ClientCertificate.CertificateSecretKeyRef, &connection.CQLClientCertificate); err != nil {
			return nil, fmt.Errorf("can't resolve CQL client certificate: %w", err)
		}
		if err := addSecretBytes(smcr.Spec.TLS.CQL.ClientCertificate.PrivateKeySecretKeyRef, &connection.CQLClientPrivateKey); err != nil {
			return nil, fmt.Errorf("can't resolve CQL client private key: %w", err)
		}
	}
	if smcr.Spec.TLS != nil && smcr.Spec.TLS.Alternator != nil {
		revision.AlternatorServerName = smcr.Spec.TLS.Alternator.ServerName
		if err := addCAValue(smcr.Spec.TLS.Alternator.CAConfigMapKeyRef, smcr.Spec.TLS.Alternator.CASecretKeyRef, &connection.AlternatorCA); err != nil {
			return nil, fmt.Errorf("can't resolve Alternator CA: %w", err)
		}
	}
	if smcr.Spec.TLS != nil && smcr.Spec.TLS.Agent != nil {
		revision.AgentServerName = smcr.Spec.TLS.Agent.ServerName
		if err := addCAValue(smcr.Spec.TLS.Agent.CAConfigMapKeyRef, smcr.Spec.TLS.Agent.CASecretKeyRef, &connection.AgentCA); err != nil {
			return nil, fmt.Errorf("can't resolve Agent CA: %w", err)
		}
	}

	sort.Slice(revision.ReferencedObjects, func(i, j int) bool {
		a, b := revision.ReferencedObjects[i], revision.ReferencedObjects[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		if a.Key != b.Key {
			return a.Key < b.Key
		}
		return a.ResourceVersion < b.ResourceVersion
	})
	connection.Revision, err = hashutil.HashObjects(revision)
	if err != nil {
		return nil, fmt.Errorf("can't calculate non-secret connection revision: %w", err)
	}

	return connection, nil
}

func scyllaDBManagerClusterName(smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration) string {
	if nameOverride, found := smcr.Annotations[naming.ScyllaDBManagerClusterRegistrationNameOverrideAnnotation]; found {
		return nameOverride
	}

	namespacePrefix := ""
	if smcr.Labels[naming.GlobalScyllaDBManagerLabel] == naming.LabelValueTrue {
		namespacePrefix = smcr.Namespace + "/"
	}
	return namespacePrefix + smcr.Spec.ScyllaDBClusterRef.Kind + "/" + smcr.Spec.ScyllaDBClusterRef.Name
}

func makeRequiredScyllaDBManagerCluster(name, ownerUID, host string, smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration, connection *resolvedScyllaDBManagerClusterConnection) *managerclientsecure.Cluster {
	cluster := &managerclientsecure.Cluster{
		Name: name, Host: host, AuthToken: connection.AuthToken,
		Username: connection.Username, Password: connection.Password,
		AlternatorAccessKeyID: connection.AlternatorAccessKeyID, AlternatorSecretAccessKey: connection.AlternatorSecretAccessKey,
		Labels:        map[string]string{naming.OwnerUIDLabel: ownerUID, naming.ManagedHash: connection.Revision},
		WithoutRepair: true,
	}
	if smcr.Spec.TLS != nil && smcr.Spec.TLS.CQL != nil {
		cluster.CQLCAFile, cluster.CQLServerName = connection.CQLCA, smcr.Spec.TLS.CQL.ServerName
		cluster.SSLUserCertFile, cluster.SSLUserKeyFile = connection.CQLClientCertificate, connection.CQLClientPrivateKey
	}
	if smcr.Spec.TLS != nil && smcr.Spec.TLS.Alternator != nil {
		cluster.AlternatorCAFile, cluster.AlternatorServerName = connection.AlternatorCA, smcr.Spec.TLS.Alternator.ServerName
	}
	if smcr.Spec.TLS != nil && smcr.Spec.TLS.Agent != nil {
		cluster.AgentCAFile, cluster.AgentServerName = connection.AgentCA, smcr.Spec.TLS.Agent.ServerName
	}
	return cluster
}

func getScyllaDBManagerCluster(ctx context.Context, smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration, managerClient *managerclientsecure.Client) (*managerclientsecure.Cluster, bool, error) {
	managerClusterName := scyllaDBManagerClusterName(smcr)
	if smcr.Status.ClusterID != nil {
		managerCluster, err := managerClient.GetCluster(ctx, *smcr.Status.ClusterID)
		if err == nil {
			return managerCluster, true, nil
		}
		if !managerclientsecure.IsNotFound(err) {
			return nil, false, err
		}
	}

	managerClusters, err := managerClient.ListClusters(ctx)
	if err != nil {
		return nil, false, err
	}
	for _, cluster := range managerClusters {
		if cluster.Name == managerClusterName {
			return cluster, true, nil
		}
	}
	return nil, false, nil
}

func setManagerAPIConnectionVerified(status *scyllav1alpha1.ScyllaDBManagerClusterRegistrationStatus, smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration, conditionStatus metav1.ConditionStatus, reason, message string) {
	apimeta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:   scyllav1alpha1.ScyllaDBManagerClusterRegistrationManagerAPIConnectionVerifiedCondition,
		Status: conditionStatus, ObservedGeneration: smcr.Generation, Reason: reason, Message: message,
	})
}

func setDatabaseConnectionVerified(status *scyllav1alpha1.ScyllaDBManagerClusterRegistrationStatus, smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration, conditionStatus metav1.ConditionStatus, reason, message string) {
	apimeta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:   scyllav1alpha1.ScyllaDBManagerClusterRegistrationDatabaseConnectionVerifiedCondition,
		Status: conditionStatus, ObservedGeneration: smcr.Generation, Reason: reason, Message: message,
	})
}

func setConnectionVerificationUnavailable(status *scyllav1alpha1.ScyllaDBManagerClusterRegistrationStatus, smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration, reason, message string) {
	setManagerAPIConnectionVerified(status, smcr, metav1.ConditionFalse, reason, message)
	setDatabaseConnectionVerified(status, smcr, metav1.ConditionFalse, reason, message)
}

func setDatabaseConnectionVerificationFromManager(status *scyllav1alpha1.ScyllaDBManagerClusterRegistrationStatus, smcr *scyllav1alpha1.ScyllaDBManagerClusterRegistration, managerStatus []*managerclientsecure.ClusterStatusItem) {
	if len(managerStatus) == 0 {
		setDatabaseConnectionVerified(status, smcr, metav1.ConditionFalse, "ManagerReportedNoNodes", "ScyllaDB Manager reported no nodes to verify.")
		return
	}

	for _, node := range managerStatus {
		if node == nil || !node.CQLTLSVerified || !node.CQLAuthVerified || !node.AgentTLSVerified {
			setDatabaseConnectionVerified(status, smcr, metav1.ConditionFalse, "DatabaseConnectionVerificationFailed", "ScyllaDB Manager has not verified CQL TLS/authentication and Agent TLS on every node.")
			return
		}
		if smcr.Spec.Authentication != nil && smcr.Spec.Authentication.Alternator != nil && (!node.AlternatorTLSVerified || !node.AlternatorAuthVerified) {
			setDatabaseConnectionVerified(status, smcr, metav1.ConditionFalse, "DatabaseConnectionVerificationFailed", "ScyllaDB Manager has not verified Alternator TLS/authentication on every node.")
			return
		}
	}

	setDatabaseConnectionVerified(status, smcr, metav1.ConditionTrue, "VerifiedDatabaseConnection", "ScyllaDB Manager verified all configured database and Agent connections on every node.")
}
