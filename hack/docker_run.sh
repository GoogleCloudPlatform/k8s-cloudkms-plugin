#!/usr/bin/env bash

IMAGE='gcr.io/google-containers/k8s-cloud-kms-plugin-test:v0.1.1'
KEY='projects/cloud-kms-lab/locations/us-central1/keyRings/ring-01/cryptoKeys/key-01'
SOCKET='@kms-plugin-socket'
HEALTHZ_PORT=8081

# Starting KMS via docker
docker run -it -p ${HEALTHZ_PORT}:${HEALTHZ_PORT} ${IMAGE} /k8s-cloud-kms-plugin --logtostderr --path-to-unix-socket=${SOCKET} --key-uri=${KEY}
