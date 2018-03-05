#!/usr/bin/env bash

TEMPLATE=kube-apiserver-template.json

ENCRYPTION_PROVIDER_CONFIG_PATH=/etc/srv/kubernetes/encryption-provider-config.yaml

KMS_SOCKET_MNT="{ \"name\": \"kmssocket\", \"mountPath\": \"/var/run/kmsplugin\", \"readOnly\": false}"
KMS_SOCKET_VOL="{ \"name\": \"kmssocket\", \"hostPath\": {\"path\": \"/var/run/kmsplugin\", \"type\": \"DirectoryOrCreate\"}}"

ENCRYPTION_PROVIDER_MNT="{ \"name\": \"encryptionconfig\", \"mountPath\": \"/etc/srv/kubernetes/encryption-provider-config.yaml\", \"readOnly\": true}"
ENCRYPTION_PROVIDER_VOL="{ \"name\": \"encryptionconfig\", \"hostPath\": {\"path\": \"/etc/srv/kubernetes/encryption-provider-config.yaml\", \"type\": \"File\"}}"


sed " {
    s@{{encryption_provider_config}}@--experimental-encryption-provider-config=${ENCRYPTION_PROVIDER_CONFIG_PATH}@

    s@{{kms_socket_mount}}@${KMS_SOCKET_MNT},@
    s@{{encryption_provider_mount}}@${ENCRYPTION_PROVIDER_MNT},@

    s@{{kms_socket_volume}}@${KMS_SOCKET_VOL},@
    s@{{encryption_provider_volume}}@${ENCRYPTION_PROVIDER_VOL},@
} " ${TEMPLATE}
