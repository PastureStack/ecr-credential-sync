# Modified by PastureStack contributors for independent maintenance and rebranding.
TARGETS := $(shell ls scripts)

DAPPER_IMAGE ?= pasturestack-ecr-credential-sync-dapper:go1.27.0-docker29.7.2-buildx0.36.1-jq1.8.1
DAPPER_HOST_ARCH ?= amd64
DOCKER_VERSION ?= 29.7.2
BUILDX_VERSION ?= 0.36.1
UBUNTU_APT_SNAPSHOT ?= 20260825T000000Z
DAPPER_SOURCE_DATE_EPOCH ?= $(shell git show -s --format=%ct HEAD)
DAPPER_BUILDX_COMMAND ?= docker buildx
DAPPER_BUILDER ?=
DAPPER_METADATA_FILE ?=
DAPPER_IID_FILE ?=
DAPPER_NO_CACHE ?= false

.PHONY: $(TARGETS) deps trash trash-keep dapper-image

dapper-image:
	bash ./scripts/normalize-runtime-context-mtimes \
		Dockerfile.dapper . $(DAPPER_SOURCE_DATE_EPOCH)
	SOURCE_DATE_EPOCH=$(DAPPER_SOURCE_DATE_EPOCH) $(DAPPER_BUILDX_COMMAND) build \
		$(if $(DAPPER_BUILDER),--builder $(DAPPER_BUILDER),) \
		$(if $(filter true,$(DAPPER_NO_CACHE)),--no-cache,) \
		$(if $(DAPPER_METADATA_FILE),--metadata-file $(DAPPER_METADATA_FILE),) \
		$(if $(DAPPER_IID_FILE),--iidfile $(DAPPER_IID_FILE),) \
		--output "type=docker,name=$(DAPPER_IMAGE),rewrite-timestamp=true,compatibility-version=20" \
		--pull \
		--provenance=false \
		--sbom=false \
		$(if $(DOCKER_BUILD_NETWORK),--network $(DOCKER_BUILD_NETWORK),) \
		--build-arg DAPPER_HOST_ARCH=$(DAPPER_HOST_ARCH) \
		--build-arg DOCKER_VERSION=$(DOCKER_VERSION) \
		--build-arg BUILDX_VERSION=$(BUILDX_VERSION) \
		--build-arg UBUNTU_APT_SNAPSHOT=$(UBUNTU_APT_SNAPSHOT) \
		--build-arg SOURCE_DATE_EPOCH=$(DAPPER_SOURCE_DATE_EPOCH) \
		-t $(DAPPER_IMAGE) \
		-f Dockerfile.dapper .

$(TARGETS): dapper-image
	docker run --rm \
		-v $(CURDIR):/go/src/github.com/PastureStack/ecr-credential-sync \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-e DAPPER_UID=$$(id -u) \
		-e DAPPER_GID=$$(id -g) \
		-e ARCH=$(DAPPER_HOST_ARCH) \
		-e TAG \
		-e REPO \
		-e IMAGE_NAMESPACE \
		-e VERSION_OVERRIDE \
		-e CROSS \
		-e WINDOWS_DOCKER_HOST \
		-e DOCKER_BUILD_NETWORK \
		-e RUNTIME_IMAGE_BUILD_TOKEN \
		$(DAPPER_IMAGE) $@

trash:
	@echo "Dependencies are vendored; no legacy trash dependency sync is required."

trash-keep: trash

deps: trash

.DEFAULT_GOAL := ci
