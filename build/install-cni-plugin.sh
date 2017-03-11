#!/usr/bin/env bash

PLUGIN_VERSION=v0.5
CNI_VERSION=v0.4.0

# Create default CNI directories.
mkdir -p /etc/cni/net.d
mkdir -p /opt/cni/bin

# Install ebtables.
if [ ! -e /sbin/ebtables ]
then
    apt-get update
    apt-get install -y ebtables
fi
/sbin/ebtables --list

# Install Azure CNI plugins.
/usr/bin/curl -sSL https://github.com/Azure/azure-container-networking/releases/download/${PLUGIN_VERSION}/azure-vnet-amd64-${PLUGIN_VERSION}.tgz > /opt/cni/bin/azure.tgz
tar -xzf /opt/cni/bin/azure.tgz -C /opt/cni/bin

# Install loopback plugin.
/usr/bin/curl -sSL https://github.com/containernetworking/cni/releases/download/${CNI_VERSION}/cni-amd64-${CNI_VERSION}.tgz > /opt/cni/bin/cni.tgz
tar -xzf /opt/cni/bin/cni.tgz -C /opt/cni/bin ./loopback

# Cleanup.
rm /opt/cni/bin/*.tgz
chown root:root /opt/cni/bin/*
