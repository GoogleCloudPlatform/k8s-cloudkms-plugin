#!/usr/bin/env bash

set -o xtrace
set -o errexit
set -o nounset

GCS_BUCKET='tpm-lab'

gsutil cp gs://"${GCS_BUCKET}"/tpmseal .
gsutil cp gs://"${GCS_BUCKET}"/tpmunseal .
chmod +x tpmseal
chmod +x tpmunseal

