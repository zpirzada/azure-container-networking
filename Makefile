SOURCEFILES = \
	$(wildcard cni/*.go) \
	$(wildcard cnm/*.go) \
	$(wildcard common/*.go) \
	$(wildcard ebtables/*.go) \
	$(wildcard ipam/*.go) \
	$(wildcard log/*.go) \
	$(wildcard netlink/*.go) \
	$(wildcard network/*.go) \
	$(wildcard store/*.go)

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
$(OUTPUTDIR)/azure-cnm-plugin: $(SOURCEFILES)
	go build -v -o $(OUTPUTDIR)/azure-cnm-plugin -ldflags "-X main.version=$(VERSION) -s -w" cnm/cnm.go

# Build the Azure CNI plugin.
$(OUTPUTDIR)/azure-cni-plugin: $(SOURCEFILES)
	go build -v -o $(OUTPUTDIR)/azure-cni-plugin -ldflags "-X main.version=$(VERSION) -s -w" cni/cni.go

install:
	go install github.com/Azure/azure-container-networking/cnm
