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

PKG=sigs.k8s.io/ibm-powervs-block-csi-driver
GIT_COMMIT?=$(shell git rev-parse --short HEAD)
BUILD_DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
REGISTRY?=gcr.io/k8s-staging-cloud-provider-ibm
IMAGE?=ibm-powervs-block-csi-driver
TAG?=$(GIT_COMMIT)
LDFLAGS?="-X ${PKG}/pkg/driver.driverVersion=${TAG} -X ${PKG}/pkg/driver.gitCommit=${GIT_COMMIT} -X ${PKG}/pkg/driver.buildDate=${BUILD_DATE} -s -w"

GO111MODULE=on
GOPROXY=direct
GOPATH=$(shell go env GOPATH)
GOOS=$(shell go env GOOS)
GOBIN=$(shell pwd)/bin

.EXPORT_ALL_VARIABLES:

bin:
	@mkdir -p $@

.PHONY: driver
bin/ibm-powervs-block-csi-driver:
driver: | bin
	CGO_ENABLED=0 GOOS=linux GOARCH=$(ARCH) go build -ldflags ${LDFLAGS} -o bin/ibm-powervs-block-csi-driver ./cmd/

.PHONY: test
test:
	go test -v -race ./cmd/... ./pkg/...

.PHONY: image
image:
	docker build -t $(REGISTRY)/$(IMAGE):$(TAG) . --target debian-base

.PHONY: push
push:
	docker push $(REGISTRY)/$(IMAGE):$(TAG)

build-image-and-push-linux-amd64: init-buildx
	{                                                                   \
	set -e ;                                                            \
	docker buildx build \
		--build-arg TARGETPLATFORM=linux/amd64 \
		-t $(REGISTRY)/$(IMAGE):$(TAG)_linux_amd64 --push . --target debian-base; \
	}

build-image-and-push-linux-ppc64le: init-buildx
	{                                                                   \
	set -e ;                                                            \
	docker buildx build \
		--build-arg TARGETPLATFORM=linux/ppc64le \
		-t $(REGISTRY)/$(IMAGE):$(TAG)_linux_ppc64le --push . --target debian-base; \
	}

build-and-push-multi-arch: build-image-and-push-linux-amd64 build-image-and-push-linux-ppc64le
	docker manifest create --amend $(REGISTRY)/$(IMAGE):$(TAG) $(REGISTRY)/$(IMAGE):$(TAG)_linux_amd64 $(REGISTRY)/$(IMAGE):$(TAG)_linux_ppc64le
	docker manifest push -p $(REGISTRY)/$(IMAGE):$(TAG)

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

.PHONY: verify-vendor
test: verify-vendor
verify: verify-vendor
verify-vendor:
	@ echo; echo "### $@:"
	@ ./hack/verify-vendor.sh

init-buildx:
	# Ensure we use a builder that can leverage it (the default on linux will not)
	-docker buildx rm multiarch-multiplatform-builder
	docker buildx create --use --name=multiarch-multiplatform-builder
	docker run --rm --privileged multiarch/qemu-user-static --reset --credential yes --persistent yes
