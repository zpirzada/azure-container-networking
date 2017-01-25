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

CNMDIR = cnm/plugin
CNI_NET_DIR = cni/network/plugin
CNI_IPAM_DIR = cni/ipam/plugin
OUTPUTDIR = out

# Containerized build parameters.
BUILD_CONTAINER_IMAGE = acn-build
BUILD_CONTAINER_NAME = acn-builder
BUILD_CONTAINER_REPO_PATH = /go/src/github.com/Azure/azure-container-networking
BUILD_USER ?= $(shell id -u)

# Docker plugin image parameters.
CNM_PLUGIN_IMAGE = ofiliz/azure-cnm-plugin
CNM_PLUGIN_ROOTFS = azure-cnm-plugin-rootfs

VERSION ?= $(shell git describe --tags --always --dirty)

ENSURE_OUTPUTDIR_EXISTS := $(shell mkdir -p $(OUTPUTDIR))

# Shorthand target names for convenience.
azure-cnm-plugin: $(OUTPUTDIR)/azure-cnm-plugin
azure-vnet: $(OUTPUTDIR)/azure-vnet
azure-vnet-ipam: $(OUTPUTDIR)/azure-vnet-ipam
azure-cni-plugin: azure-vnet azure-vnet-ipam
all-binaries: azure-cnm-plugin azure-cni-plugin

# Clean all build artifacts.
.PHONY: clean
clean:
	rm -rf $(OUTPUTDIR)

# Build the Azure CNM plugin.
$(OUTPUTDIR)/azure-cnm-plugin: $(CNMFILES)
	go build -v -o $(OUTPUTDIR)/azure-cnm-plugin -ldflags "-X main.version=$(VERSION) -s -w" $(CNMDIR)/*.go

# Build the Azure CNI network plugin.
$(OUTPUTDIR)/azure-vnet: $(CNIFILES)
	go build -v -o $(OUTPUTDIR)/azure-vnet -ldflags "-X main.version=$(VERSION) -s -w" $(CNI_NET_DIR)/*.go

# Build the Azure CNI IPAM plugin.
$(OUTPUTDIR)/azure-vnet-ipam: $(CNIFILES)
	go build -v -o $(OUTPUTDIR)/azure-vnet-ipam -ldflags "-X main.version=$(VERSION) -s -w" $(CNI_IPAM_DIR)/*.go

# Build the Azure CNI IPAM plugin for windows_amd64.
$(OUTPUTDIR)/windows_amd64/azure-vnet-ipam: $(CNIFILES)
	GOOS=windows GOARCH=amd64 go build -v -o $(OUTPUTDIR)/windows_amd64/azure-vnet-ipam -ldflags "-X main.version=$(VERSION) -s -w" $(CNI_IPAM_DIR)/*.go

# Build all binaries in a container.
.PHONY: build-containerized
build-containerized:
	pwd && ls -l
	docker build -f Dockerfile.build -t $(BUILD_CONTAINER_IMAGE):$(VERSION) .
	docker run --name $(BUILD_CONTAINER_NAME) \
		$(BUILD_CONTAINER_IMAGE):$(VERSION) \
		bash -c '\
			pwd && ls -l && \
			make all-binaries && \
			chown -R $(BUILD_USER):$(BUILD_USER) $(OUTPUTDIR) \
		'
	docker cp $(BUILD_CONTAINER_NAME):$(BUILD_CONTAINER_REPO_PATH)/$(OUTPUTDIR) .
	docker rm $(BUILD_CONTAINER_NAME)
	docker rmi $(BUILD_CONTAINER_IMAGE):$(VERSION)

# Build the Azure CNM plugin image, installable with "docker plugin install".
.PHONY: azure-cnm-plugin-image
azure-cnm-plugin-image: azure-cnm-plugin
	# Build the plugin image, keeping any old image during build for cache, but remove it afterwards.
	docker images -q $(CNM_PLUGIN_ROOTFS):$(VERSION) > cid
	docker build -f Dockerfile.cnm -t $(CNM_PLUGIN_ROOTFS):$(VERSION) .
	$(eval CID := `cat cid`)
	docker rmi $(CID) || true

	# Create a container using the image and export its rootfs.
	docker create $(CNM_PLUGIN_ROOTFS):$(VERSION) > cid
	$(eval CID := `cat cid`)
	mkdir -p $(OUTPUTDIR)/$(CID)/rootfs
	docker export $(CID) | tar -x -C $(OUTPUTDIR)/$(CID)/rootfs
	docker rm -vf $(CID)

	# Copy the plugin configuration and set ownership.
	cp cnm/config.json $(OUTPUTDIR)/$(CID)
	chgrp -R docker $(OUTPUTDIR)/$(CID)

	# Create the plugin.
	docker plugin rm $(CNM_PLUGIN_IMAGE):$(VERSION) || true
	docker plugin create $(CNM_PLUGIN_IMAGE):$(VERSION) $(OUTPUTDIR)/$(CID)

	# Cleanup temporary files.
	rm -rf $(OUTPUTDIR)/$(CID)
	rm cid

# Publish the Azure CNM plugin image to a Docker registry.
.PHONY: publish-azure-cnm-plugin-image
publish-azure-cnm-plugin-image:
	docker plugin push $(CNM_PLUGIN_IMAGE):$(VERSION)
