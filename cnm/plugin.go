// Copyright Microsoft Corp.
// All rights reserved.

package cnm

import (
	"net/http"
	"os"

	"github.com/Azure/azure-container-networking/common"
	"github.com/Azure/azure-container-networking/log"
)

// Plugin is the parent class for CNM plugins.
type Plugin struct {
	*common.Plugin
	EndpointType string
	Listener     *common.Listener
}

// Creates a new Plugin object.
func NewPlugin(name, version, endpointType string) (*Plugin, error) {
	// Setup base plugin.
	plugin, err := common.NewPlugin(name, version)
	if err != nil {
		return nil, err
	}

	return &Plugin{
		Plugin:       plugin,
		EndpointType: endpointType,
	}, nil
}

// Initializes the plugin and starts the listener.
func (plugin *Plugin) Initialize(config *common.PluginConfig) error {
	// Initialize the base plugin.
	plugin.Plugin.Initialize(config)

	// Create the plugin path.
	os.MkdirAll(pluginPath, 0660)

	// Create the listener.
	var localAddr string
	if plugin.Name != "test" {
		localAddr = config.Name + plugin.Name
	}

	listener, err := common.NewListener("unix", localAddr)
	if err != nil {
		return err
	}

	// Add generic protocol handlers.
	listener.AddHandler(activatePath, plugin.activate)

	// Start the listener.
	err = listener.Start(config.ErrChan)
	plugin.Listener = listener

	return err
}

// Uninitializes the plugin.
func (plugin *Plugin) Uninitialize() {
	plugin.Listener.Stop()
	plugin.Plugin.Uninitialize()
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
