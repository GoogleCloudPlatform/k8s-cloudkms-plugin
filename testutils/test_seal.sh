#!/usr/bin/env bash

set -o xtrace
set -o errexit
set -o nounset

SA_EXPORT=sa.json
SA_OUT=cleartext.json

./tpmseal --path-to-plaintext="${SA_EXPORT}"
./tpmunseal --path-to-output "${SA_OUT}"
cat "${SA_OUT}"