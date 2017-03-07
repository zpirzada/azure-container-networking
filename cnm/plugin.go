// Copyright 2017 Microsoft. All rights reserved.
// MIT License

package cnm

import (
	"net/http"
	"os"
	"path"

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

	if config.Listener == nil {
		// Create the plugin path.
		os.MkdirAll(pluginPath, 0660)

		// Create the listener.
		var sockName string
		if config.SockName != "" {
			sockName = config.SockName
		} else if plugin.Name != "test" {
			sockName = plugin.Name
		}

		listener, err := common.NewListener("unix", path.Join(pluginPath, sockName))
		if err != nil {
			return err
		}

		// Add generic protocol handlers.
		listener.AddHandler(activatePath, plugin.activate)

		// Start the listener.
		err = listener.Start(config.ErrChan)
		if err != nil {
			return err
		}

		config.Listener = listener
	}

	plugin.Listener = config.Listener

	return nil
}

// Uninitializes the plugin.
func (plugin *Plugin) Uninitialize() {
	plugin.Listener.Stop()
	plugin.Plugin.Uninitialize()
}

// ParseOptions returns generic options from a libnetwork request.
func (plugin *Plugin) ParseOptions(options OptionMap) OptionMap {
	opt, _ := options[genericData].(OptionMap)
	return opt
}

//
// Libnetwork remote plugin API
//

// Handles Activate requests.
func (plugin *Plugin) activate(w http.ResponseWriter, r *http.Request) {
	var req activateRequest

	log.Request(plugin.Name, &req, nil)

	resp := activateResponse{Implements: plugin.Listener.GetEndpoints()}
	err := plugin.Listener.Encode(w, &resp)

	log.Response(plugin.Name, &resp, err)
}

// Sends and logs an error response.
func (plugin *Plugin) SendErrorResponse(w http.ResponseWriter, errMsg error) {
	resp := errorResponse{errMsg.Error()}
	err := plugin.Listener.Encode(w, &resp)

	log.Response(plugin.Name, &resp, err)
}
