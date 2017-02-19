# Microsoft Azure Container Networking

## Overview
This repository is a place for the community to build the best container networking experience on Azure. It contains container networking plugins and tools for Linux and Windows containers running on Azure:
* [libnetwork (CNM) network and IPAM plugins](docs/cnm.md) for Docker Engine and Docker Swarm. 
* [CNI network and IPAM plugins](docs/cni,md) for Kubernetes and DC/OS.

Plugins implement similar functionality for their respective use cases on both platforms. The network plugins connect containers to your [Azure VNET](https://docs.microsoft.com/en-us/azure/virtual-network/virtual-networks-overview), to take advantage of Azure SDN capabilities. The IPAM plugins provide IP address management functionality for container IP addresses from Azure VNET address space.

Two environments are supported:

* [Microsoft Azure](https://azure.microsoft.com): Available in all Azure regions.
* [Microsoft Azure Stack](https://azure.microsoft.com/en-us/overview/azure-stack/): The hybrid cloud platform that enables you to deliver Azure services from your own datacenter.

Plugins are offered as part of [Azure Container Service (ACS)](https://azure.microsoft.com/en-us/services/container-service), as well as for individual Azure IaaS VMs. For ACS clusters created by [acs-engine](https://github.com/Azure/acs-engine), the deployment and configuration of both plugins on both Linux and Windows nodes is automatic.

## Documentation
See [Documentation](docs/README.md) for details, use cases and examples.

## Build
This repository builds on Windows and Linux. Build plugins directly from the source code for the latest version.

```bash
$ git clone https://github.com/Azure/azure-container-networking
$ cd azure-container-networking
$ make all-binaries
```

Then follow the instructions for the plugin in [Documentation](docs/README.md).

## Contributions
Contributions in the form of bug reports, feature requests and PRs are always welcome.

Please follow these instructions before submitting a PR:
* Create an issue describing the problem you are trying to solve.
* Clone the repository and create a topic branch.
* Make changes, adding new tests for new functionality.
* Run the checkin validation tests.
* Submit a PR.

## Changelog
See [CHANGELOG](CHANGELOG.md).

## License
See [LICENSE](LICENSE).

## Code of Conduct
This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/). For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.
