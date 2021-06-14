SHELL ?= /bin/bash

.DEFAULT_GOAL := build

################################################################################
# Version details                                                              #
################################################################################

# This will reliably return the short SHA1 of HEAD or, if the working directory
# is dirty, will return that + "-dirty"
GIT_VERSION = $(shell git describe --always --abbrev=7 --dirty --match=NeVeRmAtCh)

################################################################################
# Containerized development environment-- or lack thereof                      #
################################################################################

ifneq ($(SKIP_DOCKER),true)
	PROJECT_ROOT := $(dir $(realpath $(firstword $(MAKEFILE_LIST))))
	GO_DEV_IMAGE := brigadecore/go-tools:v0.1.0

	GO_DOCKER_CMD := docker run \
		-it \
		--rm \
		-e SKIP_DOCKER=true \
		-e GITHUB_TOKEN=$${GITHUB_TOKEN} \
		-e GOCACHE=/workspaces/brigade/.gocache \
		-v $(PROJECT_ROOT):/workspaces/brigade \
		-w /workspaces/brigade \
		$(GO_DEV_IMAGE)

	JS_DEV_IMAGE := node:14.16.0-stretch

	JS_DOCKER_CMD := docker run \
		-it \
		--rm \
		-e NPM_TOKEN=$${NPM_TOKEN} \
		-e SKIP_DOCKER=true \
		-v $(PROJECT_ROOT):/workspaces/brigade \
		-w /workspaces/brigade \
		$(JS_DEV_IMAGE)

	KANIKO_IMAGE := brigadecore/kaniko:v0.2.0

	KANIKO_DOCKER_CMD := docker run \
		-it \
		--rm \
		-e SKIP_DOCKER=true \
		-e DOCKER_PASSWORD=$${DOCKER_PASSWORD} \
		-v $(PROJECT_ROOT):/workspaces/brigade \
		-w /workspaces/brigade \
		$(KANIKO_IMAGE)

	HELM_IMAGE := brigadecore/helm-tools:v0.1.0

	HELM_DOCKER_CMD := docker run \
	  -it \
		--rm \
		-e SKIP_DOCKER=true \
		-e HELM_PASSWORD=$${HELM_PASSWORD} \
		-v $(PROJECT_ROOT):/workspaces/brigade \
		-w /workspaces/brigade \
		$(HELM_IMAGE)
endif

################################################################################
# Binaries and Docker images we build and publish                              #
################################################################################

ifdef DOCKER_REGISTRY
	DOCKER_REGISTRY := $(DOCKER_REGISTRY)/
endif

ifdef DOCKER_ORG
	DOCKER_ORG := $(DOCKER_ORG)/
endif

DOCKER_IMAGE_PREFIX := $(DOCKER_REGISTRY)$(DOCKER_ORG)brigade2-

ifdef HELM_REGISTRY
	HELM_REGISTRY := $(HELM_REGISTRY)/
endif

ifdef HELM_ORG
	HELM_ORG := $(HELM_ORG)/
endif

HELM_CHART_PREFIX := $(HELM_REGISTRY)$(HELM_ORG)

ifdef VERSION
	MUTABLE_DOCKER_TAG := latest
else
	VERSION            := $(GIT_VERSION)
	MUTABLE_DOCKER_TAG := edge
endif

IMMUTABLE_DOCKER_TAG := $(VERSION)

################################################################################
# Tests                                                                        #
################################################################################


################################################################################
# Build                                                                        #
################################################################################

.PHONY: build
build: build-images

.PHONY: build-images
build-images: build-prometheus

.PHONY: build-%
build-%:
	$(KANIKO_DOCKER_CMD) kaniko \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(GIT_VERSION) \
		--dockerfile /workspaces/brigade/v2/$*/Dockerfile \
		--context dir:///workspaces/brigade/ \
		--no-push

################################################################################
# Publish                                                                      #
################################################################################

.PHONY: publish
publish: push-images publish-chart

.PHONY: push-images
push-images: push-prometheus

.PHONY: push-%
push-%:
	$(KANIKO_DOCKER_CMD) sh -c ' \
		docker login $(DOCKER_REGISTRY) -u $(DOCKER_USERNAME) -p $${DOCKER_PASSWORD} && \
		kaniko \
			--build-arg VERSION="$(VERSION)" \
			--build-arg COMMIT="$(GIT_VERSION)" \
			--dockerfile /workspaces/brigade/v2/$*/Dockerfile \
			--context dir:///workspaces/brigade/ \
			--destination $(DOCKER_IMAGE_PREFIX)$*:$(IMMUTABLE_DOCKER_TAG) \
			--destination $(DOCKER_IMAGE_PREFIX)$*:$(MUTABLE_DOCKER_TAG) \
	'

.PHONY: publish-chart
publish-chart:
	$(HELM_DOCKER_CMD) sh	-c ' \
		helm registry login $(HELM_REGISTRY) -u $(HELM_USERNAME) -p $${HELM_PASSWORD} && \
		cd charts/brigade && \
		helm dep up && \
		sed -i "s/^version:.*/version: $(VERSION)/" Chart.yaml && \
		sed -i "s/^appVersion:.*/appVersion: $(VERSION)/" Chart.yaml && \
		helm chart save . $(HELM_CHART_PREFIX)brigade:$(VERSION) && \
		helm chart push $(HELM_CHART_PREFIX)brigade:$(VERSION) \
	'

################################################################################
# Targets to facilitate hacking on Brigade.                                    #
################################################################################

.PHONY: hack-new-kind-cluster
hack-new-kind-cluster:
	hack/kind/new-cluster.sh

.PHONY: hack-build-images
hack-build-images: hack-build-prometheus

.PHONY: hack-build-%
hack-build-%:
	docker build \
		-f v2/$*/Dockerfile \
		-t $(DOCKER_IMAGE_PREFIX)$*:$(IMMUTABLE_DOCKER_TAG) \
		--build-arg VERSION='$(VERSION)' \
		--build-arg COMMIT='$(GIT_VERSION)' \
		.

.PHONY: hack-push-images
hack-push-images: hack-push-prometheus

.PHONY: hack-push-%
hack-push-%: hack-build-%
	docker push $(DOCKER_IMAGE_PREFIX)$*:$(IMMUTABLE_DOCKER_TAG)

IMAGE_PULL_POLICY ?= Always

.PHONY: hack-deploy
hack-deploy:
	kubectl get namespace brigade || kubectl create namespace brigade
	helm dep up charts/brigade && \
	helm upgrade brigade charts/brigade \
		--install \
		--namespace brigade \
		--wait \
		--timeout 600s \
		--set prometheus.image.repository=$(DOCKER_IMAGE_PREFIX)prometheus \
		--set prometheus.image.tag=$(IMMUTABLE_DOCKER_TAG) \
		--set prometheus.image.pullPolicy=$(IMAGE_PULL_POLICY) \

.PHONY: hack
hack: hack-push-images hack-deploy

# Convenience targets for loading images into a KinD cluster
.PHONY: hack-load-images
hack-load-images: load-prometheus

load-%:
	@echo "Loading $(DOCKER_IMAGE_PREFIX)$*:$(IMMUTABLE_DOCKER_TAG)"
	@kind load docker-image $(DOCKER_IMAGE_PREFIX)$*:$(IMMUTABLE_DOCKER_TAG) \
			|| echo >&2 "kind not installed or error loading image: $(DOCKER_IMAGE_PREFIX)$*:$(IMMUTABLE_DOCKER_TAG)"

docs-stop-preview:
	@docker rm -f brigade-docs &> /dev/null || true

docs-preview: docs-stop-preview
	@docker run -d -v $$PWD:/src -p 1313:1313 --name brigade-docs -w /src/docs \
	klakegg/hugo:0.54.0-ext-alpine server -D -F --noHTTPCache --watch --bind=0.0.0.0
	# Wait for the documentation web server to finish rendering
	@until docker logs brigade-docs | grep -m 1  "Web Server is available"; do : ; done
	@open "http://localhost:1313"