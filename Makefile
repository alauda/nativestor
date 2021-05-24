## Dependency versions
CONTROLLER_TOOLS_VERSION=0.5.0


SUDO=sudo
BINDIR := $(PWD)/bin
CONTROLLER_GEN := $(BINDIR)/controller-gen
STATICCHECK := $(BINDIR)/staticcheck
INEFFASSIGN := $(BINDIR)/ineffassign


TOPOLVM_OPERATOR_VERSION ?= devel
IMAGE_TAG ?= latest


# Run tests
test: fmt vet
	$(STATICCHECK) ./...
	$(INEFFASSIGN) ./...
	$(SUDO) go test -race -v $$(go list ./... | grep -v vendor | grep -v e2e)

# Build topolvm-operator binary
build: fmt vet
	mkdir -p build
	go build -o build/topolvm -ldflags "-w -s -X github.com/alauda/topolvm-operator/main.Version=$(TOPOLVM_OPERATOR_VERSION))"  main.go

# Generate manifests e.g. CRD, RBAC etc.
manifests:
	$(CONTROLLER_GEN) \
		crd:crdVersions=v1 \
		rbac:roleName=topolvm-global \
		paths="./api/...;./controllers" \
		output:crd:artifacts:config=config/crd/bases

image:
	docker build -t $(IMAGE_PREFIX)topolvm-operator:devel --build-arg TOPOLVM_OPERATOR_VERSION=$(TOPOLVM_OPERATOR_VERSION) .

tag:
	docker tag $(IMAGE_PREFIX)topolvm-operator:devel $(IMAGE_PREFIX)topolvm-operator:$(IMAGE_TAG)

push:
	docker push $(IMAGE_PREFIX)topolvm-operator:$(IMAGE_TAG)

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate:
	$(CONTROLLER_GEN) object:headerFile=./hack/boilerplate.go.txt paths="./api/..."

setup: tools
	mkdir -p bin
	GOBIN=$(BINDIR) go install sigs.k8s.io/controller-tools/cmd/controller-gen@v$(CONTROLLER_TOOLS_VERSION)

tools:
	GOBIN=$(BINDIR) go install golang.org/x/tools/cmd/goimports@latest
	GOBIN=$(BINDIR) go install honnef.co/go/tools/cmd/staticcheck@latest
	GOBIN=$(BINDIR) go install github.com/gordonklaus/ineffassign@latest
	GOBIN=$(BINDIR) go install github.com/gostaticanalysis/nilerr/cmd/nilerr@latest


check-uncommitted:
	$(MAKE) manifests
	$(MAKE) generate
#	git diff --exit-code --name-only