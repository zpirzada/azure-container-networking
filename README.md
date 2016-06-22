# Overview
Aqua sets up networking for containers running in Azure VMs.
The current implementation does the following<br>
1. Implements the interface published by docker's libnetwork.

# Usage
aqua [net] [ipam]

# Examples
To start the remote network plugin for docker: aqua net<br>
To start the remote ipam plugin for docker: aqua ipam<br>

To create  a docker network called "azure", use the following<br>
docker network create --driver=aqua --ipam-driver=nullipam azure

Once the above network is created, you can have container join the above network as follows<br>
docker run --net=azure -it ubuntu:latest /bin/bash

#Requirements
Aqua currently relies on the fact that the interfaces in the VM are provisioned with multiple ip-addresses (called CAs in azure). In the current version of Aqua, multiple ip-addresses need to be manually configured on the interface. These ip-addresses need to be provisioned via Azure ARM APIs before they can be used inside VMs.

#Code of Conduct
This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/). For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.
