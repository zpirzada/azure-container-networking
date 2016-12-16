# Source files common to all targets.
COREFILES = \
	$(wildcard common/*.go) \
	$(wildcard ebtables/*.go) \
	$(wildcard ipam/*.go) \
	$(wildcard log/*.go) \
	$(wildcard netlink/*.go) \
	$(wildcard network/*.go) \
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
	$(wildcard cni/network/*.go) \
	$(wildcard cni/plugin/*.go) \
	$(COREFILES)

CNMDIR = cnm/plugin

CNIDIR = cni/plugin

OUTPUTDIR = out

VERSION ?= $(shell git describe --tags --always --dirty)

ENSURE_OUTPUTDIR_EXISTS := $(shell mkdir -p $(OUTPUTDIR))

# Shorthand target names for convenience.
azure-cnm-plugin: $(OUTPUTDIR)/azure-cnm-plugin
azure-cni-plugin: $(OUTPUTDIR)/azure-cni-plugin

# Clean all build artifacts.
.PHONY: clean
clean:
	rm -rf $(OUTPUTDIR)

# Build the Azure CNM plugin.
$(OUTPUTDIR)/azure-cnm-plugin: $(CNMFILES)
	go build -v -o $(OUTPUTDIR)/azure-cnm-plugin -ldflags "-X main.version=$(VERSION) -s -w" $(CNMDIR)/*.go

# Build the Azure CNI plugin.
$(OUTPUTDIR)/azure-cni-plugin: $(CNIFILES)
	go build -v -o $(OUTPUTDIR)/azure-cni-plugin -ldflags "-X main.version=$(VERSION) -s -w" $(CNIDIR)/*.go

install:
	go install github.com/Azure/azure-container-networking/cnm
