#!/usr/bin/env bash
ZONE="us-central1-b"
MASTER="kubernetes-master"
gcloud compute scp kube-apiserver.manifest ${MASTER}:/home/alextc --zone ${ZONE}
gcloud compute scp ../manifests/encryption-provider-config.yaml ${MASTER}:/home/alextc --zone ${ZONE}