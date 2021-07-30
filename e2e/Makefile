
TEST_KUBERNETES_TARGET ?= current
## Dependency versions
KIND_VERSION=0.10.0
BINDIR=$(PWD)/bin

SUDO=sudo
KIND_CLUSTER_NAME=topolvm-e2e
KIND := $(BINDIR)/kind
KUBECTL := $(BINDIR)/kubectl
GINKGO := $(BINDIR)/ginkgo


PACKAGES := unzip lvm2 xfsprogs


ifeq ($(TEST_KUBERNETES_TARGET),current)
TEST_KUBERNETES_VERSION=1.20
else ifeq ($(TEST_KUBERNETES_TARGET),prev)
TEST_KUBERNETES_VERSION=1.19
endif


export TEST_KUBERNETES_VERSION



ifeq ($(TEST_KUBERNETES_VERSION),1.20)
KUBERNETES_VERSION=1.20.2
KUBEADM_APIVERSION=kubeadm.k8s.io/v1beta2
else ifeq ($(TEST_KUBERNETES_VERSION),1.19)
KUBERNETES_VERSION=1.19.7
KUBEADM_APIVERSION=kubeadm.k8s.io/v1beta2
endif


GO_FILES := $(shell find .. -path ../vendor -prune -o -path ../e2e -prune -o -name '*.go' -print)
SERVER_CERT_FILES=./certs/cert.pem ./certs/key.pem
BACKING_STORE=./build

topolvm-operator.img: $(GO_FILES)
	rm -rf tmpbin
	mkdir -p tmpbin
	CGO_ENABLED=0 go build -o tmpbin/topolvm ../main.go
	docker build --no-cache --rm=false -f Dockerfile -t topolvm-operator:dev tmpbin
	docker save -o $@ topolvm-operator:dev

launch-kind:
	[ ! -z $(docker images -q kindest/node:tmp) ] || docker pull kindest/node:v$(KUBERNETES_VERSION) && \
	docker tag kindest/node:v$(KUBERNETES_VERSION)  kindest/node:tmp
	$(SUDO) rm -rf /tmp/topolvm/controller /tmp/topolvm/worker*
	docker build  --no-cache -t kindest/node:v$(KUBERNETES_VERSION) -f kind/Dockerfile --build-arg kindBase=kindest/node:tmp kind
	sed -e "s|@KUBERNETES_VERSION@|$(KUBERNETES_VERSION)|" \
		-e "s|@KUBEADM_APIVERSION@|$(KUBEADM_APIVERSION)|" topolvm-cluster.yaml > /tmp/topolvm-cluster.yaml
	$(KIND) create cluster --name=$(KIND_CLUSTER_NAME) --config /tmp/topolvm-cluster.yaml --image kindest/node:v$(KUBERNETES_VERSION)

shutdown-kind:
	$(KIND) delete cluster --name=$(KIND_CLUSTER_NAME) || true
	sleep 2
	for d in $$($(SUDO) find /tmp/topolvm -type d); do \
		if $(SUDO) mountpoint -q $$d; then \
			$(SUDO) umount $$d; \
		fi; \
	done
	for d in $$(mount | grep /lib/kubelet | cut -d ' ' -f 3); do $(SUDO) umount $$d; done

start-create-dev:
	mkdir -p build $(BACKING_STORE)
	if [ -f $(BACKING_STORE)/backing_store* ]; then $(MAKE) delete-dev; fi

	for i in $$(seq 4); do \
		truncate --size=5G $(BACKING_STORE)/backing_store$${i}_1; \
		$(SUDO) losetup -f $(BACKING_STORE)/backing_store$${i}_1; \
	done

	sed -e "s|@DEVICE_LOOP1@|$$($(SUDO) losetup -j $(BACKING_STORE)/backing_store1_1 | cut -d: -f1)|" \
	    -e "s|@DEVICE_LOOP2@|$$($(SUDO) losetup -j $(BACKING_STORE)/backing_store2_1 | cut -d: -f1)|" \
	    -e "s|@DEVICE_LOOP3@|$$($(SUDO) losetup -j $(BACKING_STORE)/backing_store3_1 | cut -d: -f1)|"  ./manifest/cluster.yaml > /tmp/cluster.yaml

delete-dev:
	$(MAKE) shutdown-kind
	for i in $$(seq 4); do \
		if [ -f $(BACKING_STORE)/backing_store$${i}_1 ]; then \
		$(SUDO) losetup -d $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store$${i}_1 | cut -d: -f1); \
		rm -f $(BACKING_STORE)/backing_store$${i}_1; \
		fi; \
	done

# --load_restrictor='none' allows the certificates to be loaded from a path which
# isn't a subdir of e2ebase
test: topolvm-operator.img $(SERVER_CERT_FILES)
	$(MAKE) start-create-dev
	$(MAKE) shutdown-kind
	$(MAKE) launch-kind
	$(KIND) load image-archive --name=$(KIND_CLUSTER_NAME) topolvm-operator.img
	$(KUBECTL) apply -f ./manifest/all.yaml
	$(KUBECTL) apply -f /tmp/cluster.yaml
	$(SUDO) -E env PATH=${PATH} E2ETEST=1 $(GINKGO) --failFast -v .

$(SERVER_CERT_FILES):
	mkdir -p certs
	go run ../pkg/gencert -outdir certs

clean: delete-dev
	rm -rf $(SERVER_CERT_FILES) topolvm-operator.img build/ $(BACKING_STORE)/backing_store*

setup:
	$(SUDO) apt-get update
	$(SUDO) apt-get -y install --no-install-recommends $(PACKAGES)
	mkdir -p $(BINDIR)
	curl -o $(BINDIR)/kind -sfL https://kind.sigs.k8s.io/dl/v$(KIND_VERSION)/kind-linux-amd64
	curl -o $(BINDIR)/kubectl -sfL https://storage.googleapis.com/kubernetes-release/release/v$(KUBERNETES_VERSION)/bin/linux/amd64/kubectl
	GOBIN=$(BINDIR) go install github.com/onsi/ginkgo/ginkgo@latest
	chmod a+x $(BINDIR)/kubectl $(BINDIR)/kind

.PHONY: launch-kind shutdown-kind start-create-dev delete-dev test clean setup

