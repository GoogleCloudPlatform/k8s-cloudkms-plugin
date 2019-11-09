#!/usr/bin/env bash

# Copyright 2018 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o xtrace
set -o errexit
set -o nounset

BAZEL_BIN="$(bazel info bazel-bin)"
GCS_BUCKET='tpm-lab'

bazel build //cmd/...:all

$HOME/google-cloud-sdk/bin/gsutil cp "${BAZEL_BIN}"/cmd/tpmseal/linux_amd64_stripped/tpmseal gs://"${GCS_BUCKET}"
$HOME/google-cloud-sdk/bin/gsutil cp "${BAZEL_BIN}"/cmd/tpmunseal/linux_amd64_stripped/tpmunseal gs://"${GCS_BUCKET}"
