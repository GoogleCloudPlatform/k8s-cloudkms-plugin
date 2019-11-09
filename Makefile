# Copyright 2019 Google, LLC.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

export GO111MODULE = on
export GOFLAGS = -mod=vendor

# build
build:
	@GOOS=linux GOARCH=amd64 go build \
	  -a \
		-ldflags "-s -w -extldflags 'static'"  \
		-installsuffix cgo \
		-tags netgo \
		-o build/k8s-cloudkms-plugin \
		./cmd/k8s-cloudkms-plugin/...
.PHONY: build

# deps updates all dependencies to their latest version and vendors the changes
deps:
	@go get -u -mod="" ./...
	@go mod tidy
	@go mod vendor
.PHONY: deps

# dev installs the plugin for local development
dev:
	@go install -i ./cmd/k8s-cloudkms-plugin/...
.PHONY: dev

# test runs the tests
test:
	@go test -short -parallel=40 ./...
.PHONY: test

# test runs the tests, including integration tests
test-acc:
	@go test -parallel=40 -count=1 ./...
.PHONY: test-acc
