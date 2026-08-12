ScyllaDBManagerClusterRegistration (scylla.scylladb.com/v1alpha1)
=================================================================

| **APIVersion**: scylla.scylladb.com/v1alpha1
| **Kind**: ScyllaDBManagerClusterRegistration
| **PluralName**: scylladbmanagerclusterregistrations
| **SingularName**: scylladbmanagerclusterregistration
| **Scope**: Namespaced
| **ListKind**: ScyllaDBManagerClusterRegistrationList
| **Served**: true
| **Storage**: true

Description
-----------


Specification
-------------

.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - apiVersion
     - string
     - APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources
   * - kind
     - string
     - Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
   * - :ref:`metadata<api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.metadata>`
     - object
     - 
   * - :ref:`spec<api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec>`
     - object
     - spec defines the desired state of ScyllaDBManagerClusterRegistration.
   * - :ref:`status<api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.status>`
     - object
     - status reflects the observed state of ScyllaDBManagerClusterRegistration.

.. _api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.metadata:

.metadata
^^^^^^^^^

Description
"""""""""""


Type
""""
object


.. _api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec:

.spec
^^^^^

Description
"""""""""""
spec defines the desired state of ScyllaDBManagerClusterRegistration.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`authentication<api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.authentication>`
     - object
     - authentication configures database credentials. Values are read from Secrets only while constructing a Manager request and are never persisted in this resource's status.
   * - :ref:`managerAPI<api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.managerAPI>`
     - object
     - managerAPI configures the verified mutual-TLS connection from Operator to Manager.
   * - :ref:`scyllaDBClusterRef<api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.scyllaDBClusterRef>`
     - object
     - scyllaDBClusterRef specifies the typed reference to the local ScyllaDB cluster. Supported kinds are ScyllaCluster, ScyllaDBCluster, and ScyllaDBDatacenter in the scylla.scylladb.com API group.
   * - :ref:`tls<api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.tls>`
     - object
     - tls configures endpoint-specific CA trust and server-name verification.

.. _api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.authentication:

.spec.authentication
^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
authentication configures database credentials. Values are read from Secrets only while constructing a Manager request and are never persisted in this resource's status.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`alternator<api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.authentication.alternator>`
     - object
     - alternator configures access-key authentication for Alternator.
   * - :ref:`cql<api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.authentication.cql>`
     - object
     - cql configures username and password authentication for CQL.

.. _api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.authentication.alternator:

.spec.authentication.alternator
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
alternator configures access-key authentication for Alternator.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`accessKeyIDSecretKeyRef<api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.authentication.alternator.accessKeyIDSecretKeyRef>`
     - object
     - accessKeyIDSecretKeyRef selects the Alternator access key ID from a Secret in the registration namespace.
   * - :ref:`secretAccessKeySecretKeyRef<api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.authentication.alternator.secretAccessKeySecretKeyRef>`
     - object
     - secretAccessKeySecretKeyRef selects the Alternator secret access key from a Secret in the registration namespace.

.. _api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.authentication.alternator.accessKeyIDSecretKeyRef:

.spec.authentication.alternator.accessKeyIDSecretKeyRef
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
accessKeyIDSecretKeyRef selects the Alternator access key ID from a Secret in the registration namespace.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - key
     - string
     - The key of the secret to select from.  Must be a valid secret key.
   * - name
     - string
     - Name of the referent. This field is effectively required, but due to backwards compatibility is allowed to be empty. Instances of this type with an empty value here are almost certainly wrong. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
   * - optional
     - boolean
     - Specify whether the Secret or its key must be defined

.. _api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.authentication.alternator.secretAccessKeySecretKeyRef:

.spec.authentication.alternator.secretAccessKeySecretKeyRef
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
secretAccessKeySecretKeyRef selects the Alternator secret access key from a Secret in the registration namespace.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - key
     - string
     - The key of the secret to select from.  Must be a valid secret key.
   * - name
     - string
     - Name of the referent. This field is effectively required, but due to backwards compatibility is allowed to be empty. Instances of this type with an empty value here are almost certainly wrong. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
   * - optional
     - boolean
     - Specify whether the Secret or its key must be defined

.. _api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.authentication.cql:

.spec.authentication.cql
^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
cql configures username and password authentication for CQL.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`passwordSecretKeyRef<api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.authentication.cql.passwordSecretKeyRef>`
     - object
     - passwordSecretKeyRef selects the CQL password from a Secret in the registration namespace.
   * - :ref:`usernameSecretKeyRef<api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.authentication.cql.usernameSecretKeyRef>`
     - object
     - usernameSecretKeyRef selects the CQL username from a Secret in the registration namespace.

.. _api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.authentication.cql.passwordSecretKeyRef:

.spec.authentication.cql.passwordSecretKeyRef
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
passwordSecretKeyRef selects the CQL password from a Secret in the registration namespace.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - key
     - string
     - The key of the secret to select from.  Must be a valid secret key.
   * - name
     - string
     - Name of the referent. This field is effectively required, but due to backwards compatibility is allowed to be empty. Instances of this type with an empty value here are almost certainly wrong. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
   * - optional
     - boolean
     - Specify whether the Secret or its key must be defined

.. _api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.authentication.cql.usernameSecretKeyRef:

.spec.authentication.cql.usernameSecretKeyRef
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
usernameSecretKeyRef selects the CQL username from a Secret in the registration namespace.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - key
     - string
     - The key of the secret to select from.  Must be a valid secret key.
   * - name
     - string
     - Name of the referent. This field is effectively required, but due to backwards compatibility is allowed to be empty. Instances of this type with an empty value here are almost certainly wrong. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
   * - optional
     - boolean
     - Specify whether the Secret or its key must be defined

.. _api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.managerAPI:

.spec.managerAPI
^^^^^^^^^^^^^^^^

Description
"""""""""""
managerAPI configures the verified mutual-TLS connection from Operator to Manager.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`caConfigMapKeyRef<api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.managerAPI.caConfigMapKeyRef>`
     - object
     - caConfigMapKeyRef selects the PEM CA bundle used to verify the Manager API. Mutually exclusive with caSecretKeyRef.
   * - :ref:`caSecretKeyRef<api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.managerAPI.caSecretKeyRef>`
     - object
     - caSecretKeyRef selects the PEM CA bundle used to verify the Manager API. Mutually exclusive with caConfigMapKeyRef.
   * - :ref:`clientCertificate<api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.managerAPI.clientCertificate>`
     - object
     - clientCertificate configures the client identity required by Manager mutual TLS.
   * - serverName
     - string
     - serverName is the DNS name verified against the Manager API serving certificate.
   * - url
     - string
     - url is the HTTPS base URL of the ScyllaDB Manager API.

.. _api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.managerAPI.caConfigMapKeyRef:

.spec.managerAPI.caConfigMapKeyRef
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
caConfigMapKeyRef selects the PEM CA bundle used to verify the Manager API. Mutually exclusive with caSecretKeyRef.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - key
     - string
     - The key to select.
   * - name
     - string
     - Name of the referent. This field is effectively required, but due to backwards compatibility is allowed to be empty. Instances of this type with an empty value here are almost certainly wrong. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
   * - optional
     - boolean
     - Specify whether the ConfigMap or its key must be defined

.. _api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.managerAPI.caSecretKeyRef:

.spec.managerAPI.caSecretKeyRef
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
caSecretKeyRef selects the PEM CA bundle used to verify the Manager API. Mutually exclusive with caConfigMapKeyRef.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - key
     - string
     - The key of the secret to select from.  Must be a valid secret key.
   * - name
     - string
     - Name of the referent. This field is effectively required, but due to backwards compatibility is allowed to be empty. Instances of this type with an empty value here are almost certainly wrong. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
   * - optional
     - boolean
     - Specify whether the Secret or its key must be defined

.. _api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.managerAPI.clientCertificate:

.spec.managerAPI.clientCertificate
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
clientCertificate configures the client identity required by Manager mutual TLS.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`certificateSecretKeyRef<api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.managerAPI.clientCertificate.certificateSecretKeyRef>`
     - object
     - certificateSecretKeyRef selects a PEM client certificate from a Secret in the registration namespace.
   * - :ref:`privateKeySecretKeyRef<api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.managerAPI.clientCertificate.privateKeySecretKeyRef>`
     - object
     - privateKeySecretKeyRef selects the corresponding PEM private key from a Secret in the registration namespace.

.. _api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.managerAPI.clientCertificate.certificateSecretKeyRef:

.spec.managerAPI.clientCertificate.certificateSecretKeyRef
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
certificateSecretKeyRef selects a PEM client certificate from a Secret in the registration namespace.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - key
     - string
     - The key of the secret to select from.  Must be a valid secret key.
   * - name
     - string
     - Name of the referent. This field is effectively required, but due to backwards compatibility is allowed to be empty. Instances of this type with an empty value here are almost certainly wrong. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
   * - optional
     - boolean
     - Specify whether the Secret or its key must be defined

.. _api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.managerAPI.clientCertificate.privateKeySecretKeyRef:

.spec.managerAPI.clientCertificate.privateKeySecretKeyRef
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
privateKeySecretKeyRef selects the corresponding PEM private key from a Secret in the registration namespace.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - key
     - string
     - The key of the secret to select from.  Must be a valid secret key.
   * - name
     - string
     - Name of the referent. This field is effectively required, but due to backwards compatibility is allowed to be empty. Instances of this type with an empty value here are almost certainly wrong. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
   * - optional
     - boolean
     - Specify whether the Secret or its key must be defined

.. _api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.scyllaDBClusterRef:

.spec.scyllaDBClusterRef
^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
scyllaDBClusterRef specifies the typed reference to the local ScyllaDB cluster. Supported kinds are ScyllaCluster, ScyllaDBCluster, and ScyllaDBDatacenter in the scylla.scylladb.com API group.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - kind
     - string
     - kind specifies the type of the resource.
   * - name
     - string
     - name specifies the name of the resource in the same namespace.

.. _api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.tls:

.spec.tls
^^^^^^^^^

Description
"""""""""""
tls configures endpoint-specific CA trust and server-name verification.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`agent<api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.tls.agent>`
     - object
     - agent configures strict ScyllaDB Manager Agent server certificate verification.
   * - :ref:`alternator<api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.tls.alternator>`
     - object
     - alternator configures strict Alternator server certificate verification.
   * - :ref:`cql<api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.tls.cql>`
     - object
     - cql configures strict CQL server certificate verification.

.. _api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.tls.agent:

.spec.tls.agent
^^^^^^^^^^^^^^^

Description
"""""""""""
agent configures strict ScyllaDB Manager Agent server certificate verification.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`caConfigMapKeyRef<api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.tls.agent.caConfigMapKeyRef>`
     - object
     - caConfigMapKeyRef selects a PEM CA bundle from a ConfigMap in the registration namespace. Mutually exclusive with caSecretKeyRef.
   * - :ref:`caSecretKeyRef<api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.tls.agent.caSecretKeyRef>`
     - object
     - caSecretKeyRef selects a PEM CA bundle from a Secret in the registration namespace. Mutually exclusive with caConfigMapKeyRef.
   * - serverName
     - string
     - serverName is the DNS name Manager must verify against the serving certificate.

.. _api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.tls.agent.caConfigMapKeyRef:

.spec.tls.agent.caConfigMapKeyRef
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
caConfigMapKeyRef selects a PEM CA bundle from a ConfigMap in the registration namespace. Mutually exclusive with caSecretKeyRef.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - key
     - string
     - The key to select.
   * - name
     - string
     - Name of the referent. This field is effectively required, but due to backwards compatibility is allowed to be empty. Instances of this type with an empty value here are almost certainly wrong. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
   * - optional
     - boolean
     - Specify whether the ConfigMap or its key must be defined

.. _api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.tls.agent.caSecretKeyRef:

.spec.tls.agent.caSecretKeyRef
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
caSecretKeyRef selects a PEM CA bundle from a Secret in the registration namespace. Mutually exclusive with caConfigMapKeyRef.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - key
     - string
     - The key of the secret to select from.  Must be a valid secret key.
   * - name
     - string
     - Name of the referent. This field is effectively required, but due to backwards compatibility is allowed to be empty. Instances of this type with an empty value here are almost certainly wrong. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
   * - optional
     - boolean
     - Specify whether the Secret or its key must be defined

.. _api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.tls.alternator:

.spec.tls.alternator
^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
alternator configures strict Alternator server certificate verification.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`caConfigMapKeyRef<api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.tls.alternator.caConfigMapKeyRef>`
     - object
     - caConfigMapKeyRef selects a PEM CA bundle from a ConfigMap in the registration namespace. Mutually exclusive with caSecretKeyRef.
   * - :ref:`caSecretKeyRef<api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.tls.alternator.caSecretKeyRef>`
     - object
     - caSecretKeyRef selects a PEM CA bundle from a Secret in the registration namespace. Mutually exclusive with caConfigMapKeyRef.
   * - serverName
     - string
     - serverName is the DNS name Manager must verify against the serving certificate.

.. _api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.tls.alternator.caConfigMapKeyRef:

.spec.tls.alternator.caConfigMapKeyRef
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
caConfigMapKeyRef selects a PEM CA bundle from a ConfigMap in the registration namespace. Mutually exclusive with caSecretKeyRef.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - key
     - string
     - The key to select.
   * - name
     - string
     - Name of the referent. This field is effectively required, but due to backwards compatibility is allowed to be empty. Instances of this type with an empty value here are almost certainly wrong. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
   * - optional
     - boolean
     - Specify whether the ConfigMap or its key must be defined

.. _api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.tls.alternator.caSecretKeyRef:

.spec.tls.alternator.caSecretKeyRef
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
caSecretKeyRef selects a PEM CA bundle from a Secret in the registration namespace. Mutually exclusive with caConfigMapKeyRef.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - key
     - string
     - The key of the secret to select from.  Must be a valid secret key.
   * - name
     - string
     - Name of the referent. This field is effectively required, but due to backwards compatibility is allowed to be empty. Instances of this type with an empty value here are almost certainly wrong. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
   * - optional
     - boolean
     - Specify whether the Secret or its key must be defined

.. _api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.tls.cql:

.spec.tls.cql
^^^^^^^^^^^^^

Description
"""""""""""
cql configures strict CQL server certificate verification.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - :ref:`caConfigMapKeyRef<api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.tls.cql.caConfigMapKeyRef>`
     - object
     - caConfigMapKeyRef selects a PEM CA bundle from a ConfigMap in the registration namespace. Mutually exclusive with caSecretKeyRef.
   * - :ref:`caSecretKeyRef<api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.tls.cql.caSecretKeyRef>`
     - object
     - caSecretKeyRef selects a PEM CA bundle from a Secret in the registration namespace. Mutually exclusive with caConfigMapKeyRef.
   * - serverName
     - string
     - serverName is the DNS name Manager must verify against the serving certificate.

.. _api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.tls.cql.caConfigMapKeyRef:

.spec.tls.cql.caConfigMapKeyRef
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
caConfigMapKeyRef selects a PEM CA bundle from a ConfigMap in the registration namespace. Mutually exclusive with caSecretKeyRef.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - key
     - string
     - The key to select.
   * - name
     - string
     - Name of the referent. This field is effectively required, but due to backwards compatibility is allowed to be empty. Instances of this type with an empty value here are almost certainly wrong. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
   * - optional
     - boolean
     - Specify whether the ConfigMap or its key must be defined

.. _api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.spec.tls.cql.caSecretKeyRef:

.spec.tls.cql.caSecretKeyRef
^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
caSecretKeyRef selects a PEM CA bundle from a Secret in the registration namespace. Mutually exclusive with caConfigMapKeyRef.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - key
     - string
     - The key of the secret to select from.  Must be a valid secret key.
   * - name
     - string
     - Name of the referent. This field is effectively required, but due to backwards compatibility is allowed to be empty. Instances of this type with an empty value here are almost certainly wrong. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
   * - optional
     - boolean
     - Specify whether the Secret or its key must be defined

.. _api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.status:

.status
^^^^^^^

Description
"""""""""""
status reflects the observed state of ScyllaDBManagerClusterRegistration.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - clusterID
     - string
     - clusterID reflects the internal identification number of the cluster in ScyllaDB Manager state.
   * - :ref:`conditions<api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.status.conditions[]>`
     - array (object)
     - conditions hold conditions describing ScyllaDBManagerClusterRegistration state.
   * - connectionRevision
     - string
     - connectionRevision is a non-secret digest of the desired connection metadata and referenced object revisions. Secret data is deliberately excluded.
   * - observedGeneration
     - integer
     - observedGeneration is the most recent generation observed for this ScyllaDBManagerClusterRegistration. It corresponds to the ScyllaDBManagerClusterRegistration's generation, which is updated on mutation by the API Server.

.. _api-scylla.scylladb.com-scylladbmanagerclusterregistrations-v1alpha1-.status.conditions[]:

.status.conditions[]
^^^^^^^^^^^^^^^^^^^^

Description
"""""""""""
Condition contains details for one aspect of the current state of this API Resource.

Type
""""
object


.. list-table::
   :widths: 25 10 150
   :header-rows: 1

   * - Property
     - Type
     - Description
   * - lastTransitionTime
     - string
     - lastTransitionTime is the last time the condition transitioned from one status to another. This should be when the underlying condition changed.  If that is not known, then using the time when the API field changed is acceptable.
   * - message
     - string
     - message is a human readable message indicating details about the transition. This may be an empty string.
   * - observedGeneration
     - integer
     - observedGeneration represents the .metadata.generation that the condition was set based upon. For instance, if .metadata.generation is currently 12, but the .status.conditions[x].observedGeneration is 9, the condition is out of date with respect to the current state of the instance.
   * - reason
     - string
     - reason contains a programmatic identifier indicating the reason for the condition's last transition. Producers of specific condition types may define expected values and meanings for this field, and whether the values are considered a guaranteed API. The value should be a CamelCase string. This field may not be empty.
   * - status
     - string
     - status of the condition, one of True, False, Unknown.
   * - type
     - string
     - type of condition in CamelCase or in foo.example.com/CamelCase.
