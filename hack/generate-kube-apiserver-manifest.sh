#!/usr/bin/env bash

KUBE_API_SERVER_POD_TEMPLATE=../manifests/kube-apiserver-template.json
KMS_PLUGIN_CONTAINER_TEMPLATE=../manifests/kms-plugin-container-template.json

ENCRYPTION_PROVIDER_CONFIG_PATH=/etc/srv/kubernetes/encryption-provider-config.yaml
ENCRYPTION_PROVIDER_CONFIG_FLAG="--experimental-encryption-provider-config=${ENCRYPTION_PROVIDER_CONFIG_PATH}"

KMS_SOCKET_DIR="/var/run/kmsplugin"
KMS_SOCKET_MNT="{ \"name\": \"kmssocket\", \"mountPath\": \"${KMS_SOCKET_DIR}\", \"readOnly\": false}"
KMS_SOCKET_VOL="{ \"name\": \"kmssocket\", \"hostPath\": {\"path\": \"${KMS_SOCKET_DIR}\", \"type\": \"DirectoryOrCreate\"}}"


ENCRYPTION_PROVIDER_MNT="{ \"name\": \"encryptionconfig\", \"mountPath\": \"${ENCRYPTION_PROVIDER_CONFIG_PATH}\", \"readOnly\": true}"
ENCRYPTION_PROVIDER_VOL="{ \"name\": \"encryptionconfig\", \"hostPath\": {\"path\": \"${ENCRYPTION_PROVIDER_CONFIG_PATH}\", \"type\": \"File\"}}"

KMS_KEY_URI="projects/cloud-kms-lab/locations/us-central1/keyRings/ring-01/cryptoKeys/key-01"
KMS_PATH_TO_SOCKET="${KMS_SOCKET_DIR}/socket.sock"

# TODO: Ideally, I would like to keep multi-line formatting of the container json (by removing | tr "\n" "\\n"), but this breaks sed.
KMS_PLUGIN_CONTAINER=$(echo $(sed " {
    s@{{kms_key_uri}}@${KMS_KEY_URI}@
    s@{{kms_path_to_socket}}@${KMS_PATH_TO_SOCKET}@
    s@{{kms_socket_mount}}@${KMS_SOCKET_MNT}@
} " ${KMS_PLUGIN_CONTAINER_TEMPLATE}) | tr "\n" "\\n")

sed " {
    s@{{encryption_provider_config}}@${ENCRYPTION_PROVIDER_CONFIG_FLAG}@

    s@{{kms_plugin_container}}@${KMS_PLUGIN_CONTAINER},@

    s@{{kms_socket_mount}}@${KMS_SOCKET_MNT},@
    s@{{encryption_provider_mount}}@${ENCRYPTION_PROVIDER_MNT},@

    s@{{kms_socket_volume}}@${KMS_SOCKET_VOL},@
    s@{{encryption_provider_volume}}@${ENCRYPTION_PROVIDER_VOL},@
} " ${KUBE_API_SERVER_POD_TEMPLATE}

