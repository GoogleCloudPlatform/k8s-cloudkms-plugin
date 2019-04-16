#!/usr/bin/env bash

set -o xtrace
set -o errexit
set -o nounset

GCS_BUCKET='tpm-lab'
SA_EXPORT=sa.json
SA_OUT=cleartext.json


gsutil cp gs://"${GCS_BUCKET}"/tpmseal .
gsutil cp gs://"${GCS_BUCKET}"/tpmunseal .
chmod +x tpmseal
chmod +x tpmunseal

./tpmseal --path-to-plaintext="${SA_EXPORT}"
./tpmunseal --path-to-output "${SA_OUT}"
cat "${SA_OUT}"