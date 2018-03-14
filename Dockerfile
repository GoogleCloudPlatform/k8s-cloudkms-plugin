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

FROM scratch

ARG kms_key_uri=projects/cloud-kms-lab/locations/us-central1/keyRings/ring-01/cryptoKeys/key-01
ARG socket_path=/kms-plugin.socket
ARG test_service_account_key=cloud-kms-lab-svc.json

LABEL maintainer="alextc@google.com"

COPY k8s-cloud-kms-plugin /
ADD ca-certificates.crt /etc/ssl/certs/

# Integration test (might be userfull if troubleshooting or benchmarking), remove in production.
COPY plugin.test /
ADD test.socket /tmp/

# Uncomment lines below only if testing in docker locally (not on GCE, GKE)
# This will allow application default credentails to be loaded from an exported service account key-file.
# On GKE credentials will be automatically provided via metadata server.
# see: https://cloud.google.com/docs/authentication/production#auth-cloud-implicit-go
# Fore details on how to export service account keys, see https://cloud.google.com/iam/docs/creating-managing-service-account-keys
# The service account should be granted Cloud KMS CryptoKey Encrypter/Decrypter IAM Permission.
COPY $test_service_account_key /
ENV GOOGLE_APPLICATION_CREDENTIALS=/$test_service_account_key
CMD ["/k8s-cloud-kms-plugin", "--key-uri=$kms_key_uri", "--path-to-unix-socket=$socket_path", "--logtostderr", "2>&1"]


