#!/usr/bin/env bash

TEMPLATE=kube-apiserver-template.json

ENCRYPTION_PROVIDER_CONFIG_PATH=/etc/srv/kubernetes/encryption-provider-config.yaml
ENCRYPTION_PROVIDER_CONFIG_FLAG="--experimental-encryption-provider-config=${ENCRYPTION_PROVIDER_CONFIG_PATH}"

KMS_SOCKET_DIR=/var/run/kmsplugin
KMS_SOCKET_MNT="{ \"name\": \"kmssocket\", \"mountPath\": \"${KMS_SOCKET_DIR}\", \"readOnly\": false}"
KMS_SOCKET_VOL="{ \"name\": \"kmssocket\", \"hostPath\": {\"path\": \"${KMS_SOCKET_DIR}\", \"type\": \"DirectoryOrCreate\"}}"

ENCRYPTION_PROVIDER_MNT="{ \"name\": \"encryptionconfig\", \"mountPath\": \"${ENCRYPTION_PROVIDER_CONFIG_PATH}\", \"readOnly\": true}"
ENCRYPTION_PROVIDER_VOL="{ \"name\": \"encryptionconfig\", \"hostPath\": {\"path\": \"${ENCRYPTION_PROVIDER_CONFIG_PATH}\", \"type\": \"File\"}}"

KMS_PROJECT=alextc-k8s-lab
KMS_LOCATION=global
KMS_RING=ring-01
KMS_KEY=my-key
KMS_PATH_TO_SOCKET="${KMS_SOCKET_DIR}/socket.sock"

sed " {
    s@{{encryption_provider_config}}@${ENCRYPTION_PROVIDER_CONFIG_FLAG}@

    s@{{kms_socket_mount}}@${KMS_SOCKET_MNT},@
    s@{{encryption_provider_mount}}@${ENCRYPTION_PROVIDER_MNT},@

    s@{{kms_socket_volume}}@${KMS_SOCKET_VOL},@
    s@{{encryption_provider_volume}}@${ENCRYPTION_PROVIDER_VOL},@

    s@{{kms_project}}@${KMS_PROJECT}@
    s@{{kms_location}}@${KMS_LOCATION}@
    s@{{kms_ring}}@${KMS_RING}@
    s@{{kms_key}}@${KMS_KEY}@
    s@{{kms_path_to_socket}}@${KMS_PATH_TO_SOCKET}@
} " ${TEMPLATE}
