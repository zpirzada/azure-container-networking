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
	EndpointType string
	Options      map[string]string
	Listener     *Listener
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
	listener.AddHandler(activatePath, plugin.activate)

	plugin.Listener = listener
	err = listener.Start(errChan)

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
func (plugin *Plugin) SendErrorResponse(w http.ResponseWriter, errMsg string) {
	resp := errorResponse{errMsg}
	err := plugin.Listener.Encode(w, &resp)

	log.Response(plugin.Name, &resp, err)
}
