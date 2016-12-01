// Copyright Microsoft Corp.
// All rights reserved.

package common

import (
	"github.com/Azure/azure-container-networking/store"
)

// Plugin is the parent class that implements behavior common to all plugins.
type Plugin struct {
	Name    string
	Version string
	Options map[string]string
	ErrChan chan error
	Store   store.KeyValueStore
}

// Plugin base interface.
type PluginApi interface {
	Start(*PluginConfig) error
	Stop()
	GetOption(string) string
	SetOption(string, string)
}

// Plugin common configuration.
type PluginConfig struct {
	Name    string
	Version string
	NetApi  interface{}
	IpamApi interface{}
	ErrChan chan error
	Store   store.KeyValueStore
}

// NewPlugin creates a new Plugin object.
func NewPlugin(name, version string) (*Plugin, error) {
	return &Plugin{
		Name:         name,
		Version:      version,
		Options:      make(map[string]string),
	}, nil
}

// Initialize initializes the plugin.
func (plugin *Plugin) Initialize(config *PluginConfig) error {
	plugin.ErrChan = config.ErrChan
	plugin.Store = config.Store

	return nil
}

// Uninitialize cleans up the plugin.
func (plugin *Plugin) Uninitialize() {
}

// GetOption gets the option value for the given key.
func (plugin *Plugin) GetOption(key string) string {
	return plugin.Options[key]
}

// SetOption sets the option value for the given key.
func (plugin *Plugin) SetOption(key, value string) {
	plugin.Options[key] = value
}
