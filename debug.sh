#!/usr/bin/env bash

# Starting KMS via docker
docker run -it -p 8081:8081 \
           -v /var/run/kmsplugin \
           gcr.io/cloud-kms-lab/k8s-cloud-kms-plugin:v0.1.1 /k8s-cloud-kms-plugin \
           --logtostderr \
           --path-to-unix-socket=/var/run/kmsplugin/soet.sock \
           --project-id=alextc-k8s-lab \
           --location-id=global \
           --key-ring-id=ring-01 \
           --key-id=my-key
