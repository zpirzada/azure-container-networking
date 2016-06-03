// Copyright Microsoft Corp.
// All rights reserved.

package ipam

import (
	"github.com/Azure/Aqua/log"
)

// IPAM configuration source.
type configSource interface {
	start() error
	stop()
	refresh() error
}

type configSink interface {
	setAddressSpace(*addressSpace) error
}

// Starts configuration source.
func (plugin *ipamPlugin) startSource() error {
	var err error

	switch plugin.GetOption("source") {

	case "azure", "":
		plugin.source, err = newAzureSource(configSink(plugin))

	case "mas":
		plugin.source, err = newMasSource(configSink(plugin))

	case "null":
		plugin.source, err = newNullSource(configSink(plugin))

	default:
		return errInvalidConfiguration
	}

	if plugin.source != nil {
		err = plugin.source.start()
	}

	return err
}

// Stops configuration source.
func (plugin *ipamPlugin) stopSource() {
	if plugin.source != nil {
		plugin.source.stop()
	}
}

// Signals configuration source to refresh.
func (plugin *ipamPlugin) refreshSource() {
	if plugin.source != nil {
		err := plugin.source.refresh()
		if err != nil {
			log.Printf("%s: Source refresh returned err=%v.\n", plugin.Name, err)
		}
	}
}
