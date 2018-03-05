#!/usr/bin/env bash

KUBE_API_SERVER_POD_TEMPLATE=kube-apiserver-template.json
KMS_PLUGIN_CONTAINER_TEMPLATE=kms-plugin-container-template.json

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

# TODO: Ideally I would like to keep multi-line formatting of the container, but
# this seems to break sed.
KMS_PLUGIN_CONTAINER=$(echo $(sed " {
    s@{{kms_project}}@${KMS_PROJECT}@
    s@{{kms_location}}@${KMS_LOCATION}@
    s@{{kms_ring}}@${KMS_RING}@
    s@{{kms_key}}@${KMS_KEY}@
    s@{{kms_path_to_socket}}@${KMS_PATH_TO_SOCKET}@
    s@{{kms_socket_mount}}@${KMS_SOCKET_MNT}@
} " ${KMS_PLUGIN_CONTAINER_TEMPLATE}) | tr '\n' "\\n")


sed " {
    s@{{encryption_provider_config}}@${ENCRYPTION_PROVIDER_CONFIG_FLAG}@

    s@{{kms_plugin_container}}@${KMS_PLUGIN_CONTAINER},@

    s@{{kms_socket_mount}}@${KMS_SOCKET_MNT},@
    s@{{encryption_provider_mount}}@${ENCRYPTION_PROVIDER_MNT},@

    s@{{kms_socket_volume}}@${KMS_SOCKET_VOL},@
    s@{{encryption_provider_volume}}@${ENCRYPTION_PROVIDER_VOL},@
} " ${KUBE_API_SERVER_POD_TEMPLATE}

