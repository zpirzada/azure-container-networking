# Microsoft Azure Container Networking

## Setup
The Azure VNET libnetwork plugin implements the Docker libnetwork [network] (https://github.com/docker/libnetwork/blob/master/docs/remote.md) and [IPAM] (https://github.com/docker/libnetwork/blob/master/docs/ipam.md) plugin interfaces. The plugin is available as a container, and can be installed directly from Docker plugin store. It can also be deployed manually as a binary.

## Install as a Container from Docker Plugin Store

  Download the latest official stable release from Docker plugin store.
```bash
$ docker plugin install azure/azure-cnm-plugin
```

## Install as a Binary
```bash
$ azure-cnm-plugin
```

## Build
Build the plugin for your environment.
```bash
$ git clone https://github.com/Azure/azure-container-networking
$ cd azure-container-networking
$ make azure-cnm-plugin
```

## Usage
```bash
$ azure-cnm-plugin --help

Usage: azure-cnm-plugin [OPTIONS]

Options:
  -e, --environment=azure      Set the operating environment {azure,mas}
  -l, --log-level=info         Set the logging level {debug,info}
  -t, --log-target=logfile     Set the logging target {logfile,syslog,stderr}
  -i, --ipam-query-interval    Set the IPAM plugin query interval
  -v, --version                Print version information
  -h, --help                   Print usage information
```

## Examples
To connect your containers to other resources on your Azure virtual network, you need to first create a Docker network. A network is a group of uniquely addressable endpoints that can communicate with each other. Pass the plugin name as both the network and IPAM plugin. You also need to specify the Azure VNET subnet for your network.

Create a network:
```bash
$ docker network create --driver=azure-vnet --ipam-driver=azure-vnet --subnet=[subnet] azure
```

When the command succeeds, it will return the network ID. Networks can also be listed by:
```bash
$ docker network ls
NETWORK ID          NAME                DRIVER              SCOPE
3159b0528a83        azure               azure-vnet          local
515779dadc8a        bridge              bridge              local
ed6e704a74ef        host                host                local
b35e3b663cc1        none                null                local
```

Connect containers to your network by specifying the network name when starting them.
```bash
$ docker run -it --rm --net=azure ubuntu:latest /bin/bash
```

Finally, once all containers on the network exit, you can delete the network.
```bash
$ docker network rm azure
```

All endpoints on the network must be deleted before the network itself can be deleted.
