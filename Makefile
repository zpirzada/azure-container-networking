# Source files common to all targets.
COREFILES = \
	$(wildcard common/*.go) \
	$(wildcard ebtables/*.go) \
	$(wildcard ipam/*.go) \
	$(wildcard log/*.go) \
	$(wildcard netlink/*.go) \
	$(wildcard network/*.go) \
	$(wildcard platform/*.go) \
	$(wildcard store/*.go)

# Source files for building CNM plugin.
CNMFILES = \
	$(wildcard cnm/*.go) \
	$(wildcard cnm/ipam/*.go) \
	$(wildcard cnm/network/*.go) \
	$(wildcard cnm/plugin/*.go) \
	$(COREFILES)

# Source files for building CNI plugin.
CNIFILES = \
	$(wildcard cni/*.go) \
	$(wildcard cni/ipam/*.go) \
	$(wildcard cni/ipam/plugin/*.go) \
	$(wildcard cni/network/*.go) \
	$(wildcard cni/network/plugin/*.go) \
	$(COREFILES)

# Build defaults.
GOOS ?= linux
GOARCH ?= amd64

# Build directories.
CNMDIR = cnm/plugin
CNI_NET_DIR = cni/network/plugin
CNI_IPAM_DIR = cni/ipam/plugin
OUTPUT_DIR = output
BUILD_DIR = $(OUTPUT_DIR)/$(GOOS)_$(GOARCH)

# Containerized build parameters.
BUILD_CONTAINER_IMAGE = acn-build
BUILD_CONTAINER_NAME = acn-builder
BUILD_CONTAINER_REPO_PATH = /go/src/github.com/Azure/azure-container-networking
BUILD_USER ?= $(shell id -u)

# TAR file names.
CNM_TAR_NAME = azure-vnet-cnm-$(GOOS)-$(GOARCH)-$(VERSION).tgz
CNI_TAR_NAME = azure-vnet-cni-$(GOOS)-$(GOARCH)-$(VERSION).tgz

# Docker libnetwork (CNM) plugin v2 image parameters.
CNM_PLUGIN_IMAGE = ofiliz/azure-cnm-plugin
CNM_PLUGIN_ROOTFS = azure-cnm-plugin-rootfs

VERSION ?= $(shell git describe --tags --always --dirty)

ENSURE_OUTPUT_DIR_EXISTS := $(shell mkdir -p $(OUTPUT_DIR))

# Shorthand target names for convenience.
azure-cnm-plugin: $(BUILD_DIR)/azure-cnm-plugin cnm-tar
azure-vnet: $(BUILD_DIR)/azure-vnet
azure-vnet-ipam: $(BUILD_DIR)/azure-vnet-ipam
azure-cni-plugin: azure-vnet azure-vnet-ipam cni-tar
all-binaries: azure-cnm-plugin azure-cni-plugin

# Clean all build artifacts.
.PHONY: clean
clean:
	rm -rf $(OUTPUT_DIR)

# Build the Azure CNM plugin.
$(BUILD_DIR)/azure-cnm-plugin: $(CNMFILES)
	go build -v -o $(BUILD_DIR)/azure-cnm-plugin -ldflags "-X main.version=$(VERSION) -s -w" $(CNMDIR)/*.go

# Build the Azure CNI network plugin.
$(BUILD_DIR)/azure-vnet: $(CNIFILES)
	go build -v -o $(BUILD_DIR)/azure-vnet -ldflags "-X main.version=$(VERSION) -s -w" $(CNI_NET_DIR)/*.go

# Build the Azure CNI IPAM plugin.
$(BUILD_DIR)/azure-vnet-ipam: $(CNIFILES)
	go build -v -o $(BUILD_DIR)/azure-vnet-ipam -ldflags "-X main.version=$(VERSION) -s -w" $(CNI_IPAM_DIR)/*.go

# Build all binaries in a container.
.PHONY: all-binaries-containerized
all-binaries-containerized:
	pwd && ls -l
	docker build -f Dockerfile.build -t $(BUILD_CONTAINER_IMAGE):$(VERSION) .
	docker run --name $(BUILD_CONTAINER_NAME) \
		$(BUILD_CONTAINER_IMAGE):$(VERSION) \
		bash -c '\
			pwd && ls -l && \
			export GOOS=$(GOOS) && \
			export GOARCH=$(GOARCH) && \
			make all-binaries && \
			chown -R $(BUILD_USER):$(BUILD_USER) $(BUILD_DIR) \
		'
	docker cp $(BUILD_CONTAINER_NAME):$(BUILD_CONTAINER_REPO_PATH)/$(BUILD_DIR) $(OUTPUT_DIR)
	docker rm $(BUILD_CONTAINER_NAME)
	docker rmi $(BUILD_CONTAINER_IMAGE):$(VERSION)

# Build the Azure CNM plugin image, installable with "docker plugin install".
.PHONY: azure-cnm-plugin-image
azure-cnm-plugin-image: azure-cnm-plugin
	# Build the plugin image, keeping any old image during build for cache, but remove it afterwards.
	docker images -q $(CNM_PLUGIN_ROOTFS):$(VERSION) > cid
	docker build \
		-f Dockerfile.cnm \
		-t $(CNM_PLUGIN_ROOTFS):$(VERSION) \
		--build-arg BUILD_DIR=$(BUILD_DIR) \
		.
	$(eval CID := `cat cid`)
	docker rmi $(CID) || true

	# Create a container using the image and export its rootfs.
	docker create $(CNM_PLUGIN_ROOTFS):$(VERSION) > cid
	$(eval CID := `cat cid`)
	mkdir -p $(OUTPUT_DIR)/$(CID)/rootfs
	docker export $(CID) | tar -x -C $(OUTPUT_DIR)/$(CID)/rootfs
	docker rm -vf $(CID)

	# Copy the plugin configuration and set ownership.
	cp cnm/config.json $(OUTPUT_DIR)/$(CID)
	chgrp -R docker $(OUTPUT_DIR)/$(CID)

	# Create the plugin.
	docker plugin rm $(CNM_PLUGIN_IMAGE):$(VERSION) || true
	docker plugin create $(CNM_PLUGIN_IMAGE):$(VERSION) $(OUTPUT_DIR)/$(CID)

	# Cleanup temporary files.
	rm -rf $(OUTPUT_DIR)/$(CID)
	rm cid

# Publish the Azure CNM plugin image to a Docker registry.
.PHONY: publish-azure-cnm-plugin-image
publish-azure-cnm-plugin-image:
	docker plugin push $(CNM_PLUGIN_IMAGE):$(VERSION)

# Create a CNI tarball for the current platform.
.PHONY: cni-tar
cni-tar:
	cp cni/azure.conf $(BUILD_DIR)/10-azure.conf
	chmod 0755 $(BUILD_DIR)/azure-vnet $(BUILD_DIR)/azure-vnet-ipam
	cd $(BUILD_DIR) && tar -czvf $(CNI_TAR_NAME) azure-vnet azure-vnet-ipam 10-azure.conf
	chown $(BUILD_USER):$(BUILD_USER) $(BUILD_DIR)/$(CNI_TAR_NAME)

# Create a CNM tarball for the current platform.
.PHONY: cnm-tar
cnm-tar:
	chmod 0755 $(BUILD_DIR)/azure-cnm-plugin
	cd $(BUILD_DIR) && tar -czvf $(CNM_TAR_NAME) azure-cnm-plugin
	chown $(BUILD_USER):$(BUILD_USER) $(BUILD_DIR)/$(CNM_TAR_NAME)
