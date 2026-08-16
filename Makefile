# Modified by PastureStack contributors for independent maintenance and rebranding.
TARGETS := $(shell ls scripts)

DAPPER_IMAGE ?= pasturestack-ecr-credential-sync-dapper:go1.26.6-docker29.7.2-buildx0.36.1
DAPPER_HOST_ARCH ?= amd64
DOCKER_VERSION ?= 29.7.2
BUILDX_VERSION ?= 0.36.1
UBUNTU_APT_SNAPSHOT ?= 20260808T000000Z

.PHONY: $(TARGETS) deps trash trash-keep dapper-image

dapper-image:
	docker build \
		$(if $(DOCKER_BUILD_NETWORK),--network $(DOCKER_BUILD_NETWORK),) \
		--build-arg DAPPER_HOST_ARCH=$(DAPPER_HOST_ARCH) \
		--build-arg DOCKER_VERSION=$(DOCKER_VERSION) \
		--build-arg BUILDX_VERSION=$(BUILDX_VERSION) \
		--build-arg UBUNTU_APT_SNAPSHOT=$(UBUNTU_APT_SNAPSHOT) \
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
		$(DAPPER_IMAGE) $@

trash:
	@echo "Dependencies are vendored; no legacy trash dependency sync is required."

trash-keep: trash

deps: trash

.DEFAULT_GOAL := ci
