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
GIT_COMMIT?=$(shell git rev-parse HEAD)
BUILD_DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
IMAGE?=ibm-powervs-block-csi-driver
STAGING_REGISTRY ?= rcmadhankumar
REGISTRY ?= $(STAGING_REGISTRY)
STAGING_IMAGE ?= $(STAGING_REGISTRY)/$(IMAGE)
TAG?=$(GIT_COMMIT)
STAGING_VERSION=v1.0.0
LDFLAGS?="-X ${PKG}/pkg/driver.driverVersion=${STAGING_VERSION} -X ${PKG}/pkg/driver.gitCommit=${GIT_COMMIT} -X ${PKG}/pkg/driver.buildDate=${BUILD_DATE} -s -w"

GO111MODULE=on
GOPROXY=direct
GOPATH=$(shell go env GOPATH)
GOOS=$(shell go env GOOS)
GOBIN=$(shell pwd)/bin
PLATFORM=linux/ppc64le

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

.PHONY: image-release
image-release:
	docker build build -t $(STAGING_IMAGE):$(STAGING_VERSION) . --target debian-base

.PHONY: image
image:
	docker build -t $(STAGING_IMAGE):$(STAGING_VERSION)-$(TAG) . --target debian-base

.PHONY: push-release
push-release:
	docker push $(STAGING_IMAGE):$(STAGING_VERSION)

.PHONY: push
push:
	docker push $(STAGING_IMAGE):$(STAGING_VERSION)-$(TAG)

# Build the docker image for the core CSI driver.
build-image-and-push: init-buildx
	{                                                                   \
	set -e ;                                                            \
	docker buildx build \
		--platform linux/ppc64le \
		--build-arg STAGINGVERSION=$(STAGINGVERSION) \
		--build-arg BUILDPLATFORM=linux/amd64 \
		--build-arg TARGETPLATFORM=linux/ppc64le \
		-t $(STAGING_IMAGE):$(STAGINGVERSION) --push .; \
	}

build-image-and-push-linux-amd64: init-buildx
	{                                                                   \
	set -e ;                                                            \
	docker buildx build \
		--platform linux/amd64 \
		--build-arg STAGINGVERSION=$(STAGING_VERSION) \
		--build-arg BUILDPLATFORM=linux/amd64 \
		--build-arg TARGETPLATFORM=linux/amd64 \
		-t $(STAGING_IMAGE):$(STAGING_VERSION)_linux_amd64 --push .; \
	}

build-image-and-push-linux-ppc64le: 
	{                                                                   \
	set -e ;                                                            \
	docker buildx build \
		--platform linux/ppc64le \
		--build-arg STAGINGVERSION=$(STAGING_VERSION) \
		--build-arg BUILDPLATFORM=linux/amd64 \
		--build-arg TARGETPLATFORM=linux/ppc64le \
		-t $(STAGING_IMAGE):$(STAGING_VERSION)_linux_ppc64le --push .; \
	}

build-and-push-multi-arch: build-image-and-push-linux-arm64 build-image-and-push-linux-ppc64le
	docker manifest create --amend $(STAGING_IMAGE):$(STAGING_VERSION) $(STAGING_IMAGE):$(STAGING_VERSION)-$(TAG)_linux_amd64 $(STAGING_IMAGE):$(STAGING_VERSION)-$(TAG)_linux_ppc64le
	docker manifest push -p $(STAGING_IMAGE):$(STAGING_VERSION)

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
	# Register gcloud as a Docker credential helper.
	# Required for "docker buildx build --push".
	gcloud auth configure-docker --quiet
