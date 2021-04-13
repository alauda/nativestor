
# Image URL to use all building/pushing image targets
IMG ?= controller:latest
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif


all: topolvm

# Run tests
test: fmt vet staticcheck ineffassign
	$(STATICCHECK) ./...
	$(INEFFASSIGN) ./...
	go test -race -v $$(go list ./... | grep -v vendor | grep -v e2e)

# Build topolvm binary
topolvm: generate fmt vet
	go build -o bin/topolvm main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./main.go

# Install CRDs into a cluster
install: manifests
	kustomize build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests
	kustomize build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	cd config/manager && kustomize edit set image controller=${IMG}
	kustomize build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) \
		crd:crdVersions=v1 \
		rbac:roleName=topolvm-global \
		paths="./api/...;./controllers" \
		output:crd:artifacts:config=config/crd/bases
	rm -f deploy/manifests/base/crd.yaml
	cp config/crd/bases/topolvm.cybozu.com_topolvmclusters.yaml deploy/manifests/base/crd.yaml

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the docker image
docker-build: test
	docker build . -t ${IMG}

# Push the docker image
docker-push:
	docker push ${IMG}

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.5 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

staticcheck:
ifeq (, $(shell which staticcheck))
	@{ \
	set -e ;\
	STATIC_CHECK_TMP_DIR=$$(mktemp -d) ;\
	cd $$STATIC_CHECK_TMP_DIR ;\
	go mod init tmp ;\
	go get honnef.co/go/tools/cmd/staticcheck ;\
	rm -rf $$STATIC_CHECK_TMP_DIR ;\
	}
STATICCHECK=$(GOBIN)/staticcheck
else
STATICCHECK=$(shell which staticcheck)
endif

ineffassign:
ifeq (, $(shell which ineffassign))
	@{ \
	set -e ;\
	INEFFASSIGN_TMP_DIR=$$(mktemp -d) ;\
	cd $$INEFFASSIGN_TMP_DIR ;\
	go mod init tmp ;\
	go get -u github.com/gordonklaus/ineffassign ;\
	rm -rf $$INEFFASSIGN_TMP_DIR ;\
	}
INEFFASSIGN=$(GOBIN)/ineffassign
else
INEFFASSIGN=$(shell which ineffassign)
endif
