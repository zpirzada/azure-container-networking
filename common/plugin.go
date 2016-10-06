// Copyright Microsoft Corp.
// All rights reserved.

package common

import (
	"net/http"

	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/store"
)

// Plugin base object.
type Plugin struct {
	Name         string
	Version      string
	EndpointType string
	Options      map[string]string
	Store        store.KeyValueStore
	Listener     *Listener
}

// Plugin base interface.
type PluginApi interface {
	Start(*PluginConfig) error
	Stop()
	SetOption(string, string)
}

// Plugin common configuration.
type PluginConfig struct {
	Name    string
	Version string
	NetApi  interface{}
	ErrChan chan error
	Store   store.KeyValueStore
}

// Creates a new Plugin object.
func NewPlugin(name, version, endpointType string) (*Plugin, error) {
	return &Plugin{
		Name:         name,
		Version:      version,
		EndpointType: endpointType,
		Options:      make(map[string]string),
	}, nil
}

// Initializes the plugin and starts the listener.
func (plugin *Plugin) Initialize(config *PluginConfig) error {
	var socketName string
	if plugin.Name != "test" {
		socketName = config.Name + plugin.Name
	}

	// Create the listener.
	listener, err := NewListener(socketName)
	if err != nil {
		return err
	}

	// Add generic protocol handlers.
	listener.AddHandler(activatePath, plugin.activate)

	// Initialize plugin properties.
	plugin.Listener = listener
	plugin.Store = config.Store

	// Start the listener.
	err = listener.Start(config.ErrChan)

	return err
}

// Uninitializes the plugin.
func (plugin *Plugin) Uninitialize() {
	plugin.Listener.Stop()
}

// Sets the option value for the given key.
func (plugin *Plugin) SetOption(key, value string) {
	plugin.Options[key] = value
}

// Gets the option value for the given key.
func (plugin *Plugin) GetOption(key string) string {
	return plugin.Options[key]
}

//
// Libnetwork remote plugin API
//

// Handles Activate requests.
func (plugin *Plugin) activate(w http.ResponseWriter, r *http.Request) {
	var req activateRequest

	log.Request(plugin.Name, &req, nil)

	resp := activateResponse{[]string{plugin.EndpointType}}
	err := plugin.Listener.Encode(w, &resp)

	log.Response(plugin.Name, &resp, err)
}

// Sends and logs an error response.
func (plugin *Plugin) SendErrorResponse(w http.ResponseWriter, errMsg error) {
	resp := errorResponse{errMsg.Error()}
	err := plugin.Listener.Encode(w, &resp)

	log.Response(plugin.Name, &resp, err)
}
