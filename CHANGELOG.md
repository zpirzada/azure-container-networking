# Changelog

## v0.7 (2017-03-31)
- Unified CNI network config file for Linux and Windows containers.
- Added hairpinning for Linux l2tunnel mode.
- Added master interface discovery.

## v0.6 (2017-03-13)
- Refactored all code to compile on both Windows and Linux.
- Added Windows CNI network and IPAM plugins.
- Added Linux l2tunnel mode for CNI and libnetwork.
- Added ability to request address pools on a specific interface.
- Added support for multiple subnets and gateways per network.
- Updated official plugin names and file locations.
- Separated CNI IPAM plugin to its own binary.
- Improved overall logging.
- Added containerized builds.
- Added basic documentation.
- Added license.

## v0.5 (2017-01-06)
- Added CNI support.
- Added Docker pluginv2 support.
- Various bug fixes.

## v0.4 (2016-09-30)
- Azure and Azure Stack environment support.
- Added automatic address configuration from WireServer based on environment.
- Refactored core net and IPAM logic for CNI support.
- Various bug fixes and polish.

## v0.3 (2016-07-29)
- Initial release of network and IPAM plugin.
- Added libnetwork plugin support.
- Added JSON store support.
