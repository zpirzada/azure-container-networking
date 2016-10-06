SOURCEFILES = \
	$(wildcard cnm/*.go) \
	$(wildcard common/*.go) \
	$(wildcard ebtables/*.go) \
	$(wildcard ipam/*.go) \
	$(wildcard log/*.go) \
	$(wildcard netlink/*.go) \
	$(wildcard network/*.go) \
	$(wildcard store/*.go)

OUTPUTDIR = dist

VERSION ?= $(shell git describe --tags --always --dirty)

# Shorthand target names for convenience.
azure-cnm-plugin: $(OUTPUTDIR)/azure-cnm-plugin

# Clean all build artifacts.
.PHONY: clean
clean:
	rm -rf $(OUTPUTDIR)

# Build the Azure CNM plugin.
dist/azure-cnm-plugin: $(SOURCEFILES)
	go build -v -o azure-cnm-plugin -ldflags "-X main.version=$(VERSION) -s -w" cnm/cnm.go

install:
	go install github.com/Azure/azure-container-networking/cnm
