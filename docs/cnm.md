# Microsoft Azure Container Networking

## Setup
The Azure VNET libnetwork plugin implements the [libnetwork network and IPAM plugin interfaces](https://github.com/docker/libnetwork/blob/master/docs/remote.md). The plugin is available as a container, and can be installed directly from Docker plugin store. It can also be deployed manually as a binary.

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
$ make azure-cnm-plugin
```

## Usage
```bash
azure-cnm-plugin --help
```

```bash
Usage: azure-cnm-plugin [OPTIONS]

Options:
  -e, --environment={azure|mas}         Set the operating environment.
  -l, --log-level={info|debug}          Set the logging level.
  -t, --log-target={syslog|stderr}      Set the logging target.
  -?, --help                            Print usage and version information.
```

## Examples
To connect your containers to other resources on your Azure virtual network, you need to first create a Docker network. A network is a group of uniquely addressable endpoints that can communicate with each other.

Create a network:
```bash
$ docker network create --driver=azurenet --ipam-driver=azureipam azure
```

When the command succeeds, it will return the network ID. Networks can also be listed by:
```bash
$ docker network ls
NETWORK ID          NAME                DRIVER              SCOPE
3159b0528a83        azure               azurenet            local
515779dadc8a        bridge              bridge              local
ed6e704a74ef        host                host                local
b35e3b663cc1        none                null                local
```

Connect containers to your network by specifying the network name when starting them.
```bash
$ docker run --net=azure -it ubuntu:latest /bin/bash
```

Finally, once all containers on the network exit, you can delete the network.
```bash
$ docker network rm azure
```

All endpoints on the network must be deleted before the network itself can be deleted.
