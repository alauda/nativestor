CONTROLLER_TOOLS_VERSION=0.7.0
KUSTOMIZE_VERSION=3.8.7

TOPOLVM_OPERATOR_VERSION ?= devel

SUDO=sudo
BINDIR := $(PWD)/bin
CSI_VERSION=1.5.0
PROTOC := PATH=$(BINDIR):$(PATH) $(BINDIR)/protoc -I=$(shell pwd)/include:.
CURL=curl -Lsf
PROTOC_VERSION=3.15.0
CONTROLLER_GEN := $(BINDIR)/controller-gen
STATICCHECK := $(BINDIR)/staticcheck
INEFFASSIGN := $(BINDIR)/ineffassign

# CHANNELS define the bundle channels used in the bundle.
# Add a new line here if you would like to change its default config. (E.g CHANNELS = "candidate,fast,stable")
# To re-generate a bundle for other specific channels without changing the standard setup, you can:
# - use the CHANNELS as arg of the bundle target (e.g make bundle CHANNELS=candidate,fast,stable)
# - use environment variables to overwrite this value (e.g export CHANNELS="candidate,fast,stable")
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif

# DEFAULT_CHANNEL defines the default channel used in the bundle.
# Add a new line here if you would like to change its default config. (E.g DEFAULT_CHANNEL = "stable")
# To re-generate a bundle for any other default channel without changing the default setup, you can:
# - use the DEFAULT_CHANNEL as arg of the bundle target (e.g make bundle DEFAULT_CHANNEL=stable)
# - use environment variables to overwrite this value (e.g export DEFAULT_CHANNEL="stable")
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

REGISTRY_PREFIX ?= docker.io
IMAGE_PREFIX ?= alaudapublic/
IMAGE_TAG_BASE ?= $(IMAGE_PREFIX)topolvm-operator
BUNDLE_IMG ?= $(IMAGE_TAG_BASE)-bundle:$(TOPOLVM_OPERATOR_VERSION)
IMAGE ?= $(REGISTRY_PREFIX)/$(IMAGE_TAG_BASE):$(TOPOLVM_OPERATOR_VERSION)
IMG ?= $(REGISTRY_PREFIX)/$(BUNDLE_IMG)

# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
# ENVTEST_K8S_VERSION = 1.22

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif



# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) \
		crd:crdVersions=v1 \
		rbac:roleName=topolvm-global \
		paths="./apis/...;./pkg/operator/topolvm/controller" \
		output:crd:artifacts:config=config/crd/bases

generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./api/..."

fmt: ## Run go fmt against code.
	go fmt ./...

vet: ## Run go vet against code.
	go vet ./...

test: manifests generate fmt vet envtest ## Run tests.
	@KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test ./... -coverprofile cover.out
	$(STATICCHECK) ./...
	$(INEFFASSIGN) ./...
	$(SUDO) go test -race -v $$(go list ./... | grep -v vendor | grep -v e2e)

example: manifests generate ## Generate yaml manifests for operator deployment
	mkdir -p deploy/example
	@echo Change value of operator-hub in manager manifest
	$(KUSTOMIZE) build config/default | sed -e 's,value: "1",value: "0",' > deploy/example/operator.yaml
	$(KUSTOMIZE) build config/overlays/ocp | sed -e 's,value: "1",value: "0",' > deploy/example/operator-ocp.yaml

setup: tools controller-gen ## Install controller-tools.

tools: ## Install tools required for testing.
	curl -sfL -o protoc.zip https://github.com/protocolbuffers/protobuf/releases/download/v$(PROTOC_VERSION)/protoc-$(PROTOC_VERSION)-osx-x86_64.zip
	unzip -o protoc.zip bin/protoc 'include/*'
	rm -f protoc.zip
	GOBIN=$(BINDIR) go install golang.org/x/tools/cmd/goimports@latest
	GOBIN=$(BINDIR) go install honnef.co/go/tools/cmd/staticcheck@latest
	GOBIN=$(BINDIR) go install github.com/gordonklaus/ineffassign@latest
	GOBIN=$(BINDIR) go install github.com/gostaticanalysis/nilerr/cmd/nilerr@latest

##@ Build

build: fmt vet ## Build topolvm-operator binary.
	go build -o bin/topolvm -ldflags "-w -s -X github.com/alauda/topolvm-operator/main.Version=$(TOPOLVM_OPERATOR_VERSION))"  main.go

run: manifests generate fmt vet ## Run a controller from your host (Not Applicable).
	go run ./main.go

docker-build: test ## Build docker image with the manager.
	docker build -t ${IMG} .

docker-push: ## Push docker image with the manager.
	docker push ${IMG}

image: ## Build docker image of topolvm-operator:devel.
	docker build -t $(IMAGE_PREFIX)topolvm-operator:devel --build-arg TOPOLVM_OPERATOR_VERSION=$(TOPOLVM_OPERATOR_VERSION) .

tag: ## Tag docker image of topolvm-operator:devel.
	docker tag $(IMAGE_PREFIX)topolvm-operator:devel $(IMAGE_PREFIX)topolvm-operator:$(IMAGE_TAG)

push: ## Push tagged docker image of topolvm-operator
	docker push $(IMAGE_PREFIX)topolvm-operator:$(IMAGE_TAG)

##@ Deployment

install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config. (Not Applicable)
	@cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. (Not Applicable)
	$(KUSTOMIZE) build config/default | kubectl delete -f -

CONTROLLER_GEN = $(shell pwd)/bin/controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v$(CONTROLLER_TOOLS_VERSION))

KUSTOMIZE = $(shell pwd)/bin/kustomize
kustomize: ## Download kustomize locally if necessary.
	$(call go-get-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v3@v$(KUSTOMIZE_VERSION))

ENVTEST = $(shell pwd)/bin/setup-envtest
envtest: ## Download envtest-setup locally if necessary.
	$(call go-get-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest@latest)

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go get $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef

.PHONY: bundle
bundle: manifests kustomize ## Generate bundle manifests and metadata, then validate generated files.
	operator-sdk generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | operator-sdk generate bundle -q --overwrite --version $(TOPOLVM_OPERATOR_VERSION) $(BUNDLE_METADATA_OPTS)
	operator-sdk bundle validate ./bundle

.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Push the bundle image.
	$(MAKE) docker-push IMG=$(BUNDLE_IMG)

.PHONY: opm
OPM = ./bin/opm
opm: ## Download opm locally if necessary.
ifeq (,$(wildcard $(OPM)))
ifeq (,$(shell which opm 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPM)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPM) https://github.com/operator-framework/operator-registry/releases/download/v1.15.1/$${OS}-$${ARCH}-opm ;\
	chmod +x $(OPM) ;\
	}
else
OPM = $(shell which opm)
endif
endif

# A comma-separated list of bundle images (e.g. make catalog-build BUNDLE_IMGS=example.com/operator-bundle:v0.1.0,example.com/operator-bundle:v0.2.0).
# These images MUST exist in a registry and be pull-able.
BUNDLE_IMGS ?= $(BUNDLE_IMG)

# The image tag given to the resulting catalog image (e.g. make catalog-build CATALOG_IMG=example.com/operator-catalog:v0.2.0).
CATALOG_IMG ?= $(IMAGE_TAG_BASE)-catalog:$(TOPOLVM_OPERATOR_VERSION)

# Set CATALOG_BASE_IMG to an existing catalog image tag to add $BUNDLE_IMGS to that image.
ifneq ($(origin CATALOG_BASE_IMG), undefined)
FROM_INDEX_OPT := --from-index $(CATALOG_BASE_IMG)
endif

# Build a catalog image by adding bundle images to an empty catalog using the operator package manager tool, 'opm'.
# This recipe invokes 'opm' in 'semver' bundle add mode. For more information on add modes, see:
# https://github.com/operator-framework/community-operators/blob/7f1438c/docs/packaging-operator.md#updating-your-existing-operator
.PHONY: catalog-build
catalog-build: opm ## Build a catalog image.
	$(OPM) index add --container-tool docker --mode semver --tag $(CATALOG_IMG) --bundles $(BUNDLE_IMGS) $(FROM_INDEX_OPT)

# Push the catalog image.
.PHONY: catalog-push
catalog-push: ## Push a catalog image.
	$(MAKE) docker-push IMG=$(CATALOG_IMG)

build/raw-device:
	mkdir -p build
	go build -o $@  ./pkg/raw_device/raw_device

.PHONY: csi.proto
csi.proto:
	$(CURL) -o $@ https://raw.githubusercontent.com/container-storage-interface/spec/v$(CSI_VERSION)/csi.proto
	sed -i 's,^option go_package.*$$,option go_package = "github.com/alauda/topolvm-operator/csi";,' csi.proto
	sed -i '/^\/\/ Code generated by make;.*$$/d' csi.proto

.PHONY: csi/csi.pb.go
csi/csi.pb.go: csi.proto
	mkdir -p csi
	$(PROTOC) --go_out=module=github.com/alauda/topolvm-operator:. $<

.PHONY: csi/csi_grpc.pb.go
csi/csi_grpc.pb.go: csi.proto
	mkdir -p csi
	$(PROTOC) --go-grpc_out=module=github.com/alauda/topolvm-operator:. $<

