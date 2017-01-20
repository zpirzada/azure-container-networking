// Copyright Microsoft Corp.
// All rights reserved.

package cni

import (
	"github.com/Azure/azure-container-networking/common"
	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/store"
)

// Plugin is the parent class for CNI plugins.
type Plugin struct {
	*common.Plugin
}

// NewPlugin creates a new CNI plugin.
func NewPlugin(name, version string) (*Plugin, error) {
	// Setup base plugin.
	plugin, err := common.NewPlugin(name, version)
	if err != nil {
		return nil, err
	}

	return &Plugin{
		Plugin: plugin,
	}, nil
}

// Initialize initializes the plugin.
func (plugin *Plugin) Initialize(config *common.PluginConfig) error {
	// Initialize the base plugin.
	plugin.Plugin.Initialize(config)

	// Initialize logging.
	log.SetName(plugin.Name)
	log.SetLevel(log.LevelInfo)
	err := log.SetTarget(log.TargetLogfile)
	if err != nil {
		log.Printf("[cni] Failed to configure logging, err:%v.\n", err)
		return err
	}

	// Initialize store.
	if plugin.Store == nil {
		// Create the key value store.
		var err error
		plugin.Store, err = store.NewJsonFileStore("/etc/cni/" + plugin.Name + ".conf")
		if err != nil {
			log.Printf("[cni] Failed to create store, err:%v.", err)
			return err
		}

		// Acquire store lock.
		err = plugin.Store.Lock(true)
		if err != nil {
			log.Printf("[cni] Timed out on locking store, err:%v.", err)
			return err
		}

		config.Store = plugin.Store
	}

	log.Printf("[cni] Plugin %v version %v initialized.", plugin.Name, plugin.Version)

	return nil
}

// Uninitialize uninitializes the plugin.
func (plugin *Plugin) Uninitialize() {
	err := plugin.Store.Unlock()
	if err != nil {
		log.Printf("[cni] Failed to unlock store, err:%v.", err)
	}

	plugin.Plugin.Uninitialize()
}
