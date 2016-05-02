// Copyright Microsoft Corp.
// All rights reserved.

package common

import (
	"net/http"

	"github.com/Azure/Aqua/log"
)

// Plugin object and interface
type Plugin struct {
	Name         string
	Version      string
	Scope        string
	EndpointType string
	Listener     *Listener
}

// Creates a new Plugin object.
func NewPlugin(name, version, scope, endpointType string) (*Plugin, error) {
	return &Plugin{
		Name:         name,
		Version:      version,
		Scope:        scope,
		EndpointType: endpointType,
	}, nil
}

// Initializes the plugin and starts the listener.
func (plugin *Plugin) Initialize(errChan chan error) error {
	var socketName string
	if plugin.Name != "test" {
		socketName = plugin.Name
	}

	// Create the listener.
	listener, err := NewListener(socketName)
	if err != nil {
		return err
	}

	// Add generic protocol handlers.
	listener.AddHandler("Plugin", "Activate", plugin.activatePlugin)

	plugin.Listener = listener
	err = listener.Start(errChan)

	return err
}

// Uninitializes the plugin.
func (plugin *Plugin) Uninitialize() {
	plugin.Listener.Stop()
}

//
// Libnetwork remote plugin API
//

func (plugin *Plugin) activatePlugin(w http.ResponseWriter, r *http.Request) {
	log.Request(plugin.Name, "Activate", nil, nil)

	resp := &activateResponse{[]string{plugin.EndpointType}}
	err := plugin.Listener.Encode(w, resp)

	log.Response(plugin.Name, "Activate", resp, err)
}
