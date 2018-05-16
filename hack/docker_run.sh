#!/usr/bin/env bash

# Starting KMS via docker
docker run -it -p 8081:8081 \
           -v /var/run/kmsplugin \
           gcr.io/google-containers/k8s-cloud-kms-plugin-test:v0.1.1 /k8s-cloud-kms-plugin \
           --logtostderr \
           --path-to-unix-socket=@kms-plugin-socket \
           --key-uri=projects/cloud-kms-lab/locations/us-central1/keyRings/ring-01/cryptoKeys/key-01
