# Copyright 2019 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

PKG=github.com/ppc64le-cloud/powervs-csi-driver
IMAGE?=quay.io/powercloud/powervs-csi-driver
VERSION=v0.0.1
GIT_COMMIT?=$(shell git rev-parse HEAD)
BUILD_DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS?="-X ${PKG}/pkg/driver.driverVersion=${VERSION} -X ${PKG}/pkg/driver.gitCommit=${GIT_COMMIT} -X ${PKG}/pkg/driver.buildDate=${BUILD_DATE} -s -w"
GO111MODULE=on
GOPROXY=direct
GOPATH=$(shell go env GOPATH)
GOOS=$(shell go env GOOS)
GOBIN=$(shell pwd)/bin
PLATFORM=linux/ppc64le

.EXPORT_ALL_VARIABLES:

.PHONY: bin/powervs-csi-driver
bin/powervs-csi-driver: | bin
	CGO_ENABLED=0 GOOS=linux GOARCH=ppc64le go build -ldflags ${LDFLAGS} -o bin/powervs-csi-driver ./cmd/

.PHONY: test
test:
	go test -v -race ./cmd/... ./pkg/...

.PHONY: image-release
image-release:
	docker buildx build -t $(IMAGE):$(VERSION) . --target debian-base

.PHONY: image
image:
	docker build -t $(IMAGE):latest . --target debian-base

.PHONY: push-release
push-release:
	docker push $(IMAGE):$(VERSION)

.PHONY: push
push:
	docker push $(IMAGE):latest

.PHONY: clean
clean:
	rm -rf bin/*

bin/mockgen: | bin
	go install github.com/golang/mock/mockgen@v1.6.0

bin/golangci-lint: | bin
	echo "Installing golangci-lint..."
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v1.43.0

mockgen: bin/mockgen
	./hack/update-gomock

.PHONY: verify
verify: bin/golangci-lint
	echo "verifying and linting files ..."
	./hack/verify-all
	echo "Congratulations! All Go source files have been linted."


.PHONY: test-e2e
test-e2e:
	TEST_PATH=./tests/e2e/... \
	GINKGO_FOCUS="\[ebs-csi-e2e\]" \
	./hack/e2e/run.sh

.PHONY: verify-vendor
test: verify-vendor
verify: verify-vendor
verify-vendor:
	@ echo; echo "### $@:"
	@ ./hack/verify-vendor.sh