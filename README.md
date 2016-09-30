## Overview
Aqua is a libnetwork plugin for containers running on Microsoft Azure. It enables containers to take advantage of Microsoft Azure SDN capabilities.

Note this plugin is still in development and will change based on feedback.

## Setup
Download the latest stable release from Docker plugin store.
```bash
$ docker plugin pull azure/aqua
```

## Supported Environments
[Microsoft Azure](https://azure.microsoft.com)<br>
[Microsoft Azure Stack](https://azure.microsoft.com/en-us/overview/azure-stack/)

## Usage
```bash
Usage: aqua [OPTIONS]

Options:
  -e, --environment={azure|mas}         Set the operating environment.
  -l, --log-level={info|debug}          Set the logging level.
  -t, --log-target={syslog|stderr}      Set the logging target.
  -?, --help                            Print usage and version information.
```

## Examples
To connect your containers to other resources on your Azure virtual network, you need to first create a Docker network. A network is a group of uniquely addressable endpoints that can communicate with each other.

Create a network:<br>
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

Connect containers to your network by specifying the network name when starting them:<br>
```bash
$ docker run --net=azure -it ubuntu:latest /bin/bash
```

Finally, once all containers on the network exit, you can delete the network:
```bash
$ docker network rm azure
```

All endpoints on the network must be deleted before the network itself can be deleted.

## Topology
The plugin creates a bridge for each underlying Azure virtual network. The bridge functions in L2 mode and is bridged to the host network interface. The bridge itself can also be assigned an IP address, turning it into a gateway for containers.

If the container host VM has multiple network interfaces, the primary network interface is reserved for management traffic. A secondary interface is used for container traffic whenever possible.

## Changelog
See [CHANGELOG](CHANGELOG.md)

## Code of Conduct
This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/). For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.
