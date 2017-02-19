# Microsoft Azure Container Networking

## Deployment Options
Azure network plugins can be configured to operate in two modes:
* `l2-tunnel`: This operation mode connects all containers to Azure VNET as a first-class citizen. All Azure SDN features that are available to VMs are also available to containers. This is the recommended and default option.

* `l2-bridge`: This operation mode offers better networking performance because traffic between two containers on the same host do not need to be forwarded to the Azure SDN stack for policy enforcement. Use only when your deployment does not use Azure SDN policies, or a 3rd party container networking policy solution is used instead.

## Network Topology
Network plugins bring both Windows and Linux containers to a single flat L3 Azure subnet. This enables full integration with other SDN features such as network security groups and load balancers.

The plugin creates a bridge for each underlying Azure VNET. The bridge functions in L2 mode and is connected to the host network interface. The bridge itself can also be assigned an IP address, turning it into a gateway for containers.

If the container host VM has multiple network interfaces, the primary network interface is reserved for management traffic. A secondary interface is used for container traffic whenever possible.
