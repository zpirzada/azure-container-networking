# Overview
Aqua is a libnetwork plugin that allows you to take advantage of the power of the Microsoft Azure SDN capabilities for your containers running in Azure.

## Usage
aqua [net] [ipam]

## Examples
Create a network with aqua:<br>
```bash
docker network create --driver=aquanet --ipam-driver=aquaipam azure
```

Connect containers to your network by specifying the network name when starting them:<br>
```bash
docker run --net=azure -it ubuntu:latest /bin/bash
```

## Code of Conduct
This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/). For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.
