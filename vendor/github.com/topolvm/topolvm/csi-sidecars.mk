# CSI sidecar versions
EXTERNAL_PROVISIONER_VERSION = 2.1.0
EXTERNAL_ATTACHER_VERSION = 3.1.0
EXTERNAL_RESIZER_VERSION = 1.1.0
NODE_DRIVER_REGISTRAR_VERSION = 2.1.0
LIVENESSPROBE_VERSION = 2.2.0
CSI_SIDECARS = \
	external-provisioner \
	external-attacher \
	external-resizer \
	node-driver-registrar \
	livenessprobe

GOPATH ?= $(shell go env GOPATH)
GO111MODULE = on
export GO111MODULE

SRC_ROOT = $(GOPATH)/src/github.com/kubernetes-csi
EXTERNAL_PROVISIONER_SRC  = $(SRC_ROOT)/external-provisioner
NODE_DRIVER_REGISTRAR_SRC = $(SRC_ROOT)/node-driver-registrar
EXTERNAL_ATTACHER_SRC     = $(SRC_ROOT)/external-attacher
EXTERNAL_RESIZER_SRC      = $(SRC_ROOT)/external-resizer
LIVENESSPROBE_SRC         = $(SRC_ROOT)/livenessprobe

OUTPUT_DIR ?= .

build: $(CSI_SIDECARS)

external-provisioner:
	rm -rf $(EXTERNAL_PROVISIONER_SRC)
	mkdir -p $(EXTERNAL_PROVISIONER_SRC)
	curl -sSLf https://github.com/kubernetes-csi/external-provisioner/archive/v$(EXTERNAL_PROVISIONER_VERSION).tar.gz | \
        tar zxf - --strip-components 1 -C $(EXTERNAL_PROVISIONER_SRC)
	make -C $(EXTERNAL_PROVISIONER_SRC)
	cp -f $(EXTERNAL_PROVISIONER_SRC)/bin/csi-provisioner $(OUTPUT_DIR)/

external-attacher:
	rm -rf $(EXTERNAL_ATTACHER_SRC)
	mkdir -p $(EXTERNAL_ATTACHER_SRC)
	curl -sSLf https://github.com/kubernetes-csi/external-attacher/archive/v$(EXTERNAL_ATTACHER_VERSION).tar.gz | \
        tar zxf - --strip-components 1 -C $(EXTERNAL_ATTACHER_SRC)
	make -C $(EXTERNAL_ATTACHER_SRC)
	cp -f $(EXTERNAL_ATTACHER_SRC)/bin/csi-attacher $(OUTPUT_DIR)/

external-resizer:
	rm -rf $(EXTERNAL_RESIZER_SRC)
	mkdir -p $(EXTERNAL_RESIZER_SRC)
	curl -sSLf https://github.com/kubernetes-csi/external-resizer/archive/v$(EXTERNAL_RESIZER_VERSION).tar.gz | \
        tar zxf - --strip-components 1 -C $(EXTERNAL_RESIZER_SRC)
	make -C $(EXTERNAL_RESIZER_SRC)
	cp -f $(EXTERNAL_RESIZER_SRC)/bin/csi-resizer $(OUTPUT_DIR)/

node-driver-registrar:
	rm -rf $(NODE_DRIVER_REGISTRAR_SRC)
	mkdir -p $(NODE_DRIVER_REGISTRAR_SRC)
	curl -sSLf https://github.com/kubernetes-csi/node-driver-registrar/archive/v$(NODE_DRIVER_REGISTRAR_VERSION).tar.gz | \
        tar zxf - --strip-components 1 -C $(NODE_DRIVER_REGISTRAR_SRC)
	make -C $(NODE_DRIVER_REGISTRAR_SRC)
	cp -f $(NODE_DRIVER_REGISTRAR_SRC)/bin/csi-node-driver-registrar $(OUTPUT_DIR)/

livenessprobe:
	rm -rf $(LIVENESSPROBE_SRC)
	mkdir -p $(LIVENESSPROBE_SRC)
	curl -sSLf https://github.com/kubernetes-csi/livenessprobe/archive/v$(LIVENESSPROBE_VERSION).tar.gz | \
        tar zxf - --strip-components 1 -C $(LIVENESSPROBE_SRC)
	make -C $(LIVENESSPROBE_SRC)
	cp -f $(LIVENESSPROBE_SRC)/bin/livenessprobe $(OUTPUT_DIR)/

.PHONY: build $(CSI_SIDECARS)
