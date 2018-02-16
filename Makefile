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

all: build

ENVVAR = GOOS=linux GOARCH=amd64 CGO_ENABLED=0
REGISTRY = gcr.io/cloud-kms-lab
IMAGE = k8s-cloud-kms-plugin
TAG = v0.1.1
BIN = k8s-cloud-kms-plugin
PROJECT = alextc-k8s-lab
ZONE = us-central1-b
MASTER = kubernetes-master

deps:
	go get github.com/tools/godep
	godep save

build: clean deps
	$(ENVVAR) godep go test ./...
	$(ENVVAR) godep go build -o $(BIN)
	$(ENVVAR) godep go test ./plugin -c

container: build
	docker build --pull --no-cache -t ${REGISTRY}/$(IMAGE):$(TAG) .

push: container
	gcloud docker -- push ${REGISTRY}/$(IMAGE):$(TAG)

copy: push
    gcloud compute scp kube-apiserver.manifest $(MASTER):/home/alextc --zone $(ZONE)

clean:
	rm -f $(BIN)