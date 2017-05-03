// Copyright 2017 Microsoft. All rights reserved.
// MIT License

// +build windows

package cnm

import (
	"os"

	"github.com/Azure/azure-container-networking/common"
)

const (
	// Default API server URL.
	defaultAPIServerURL = "tcp://localhost:48080"

	// Docker plugin paths.
	pluginSpecPath = "\\docker\\plugins\\"
)

// GetAPIServerURL returns the API server URL.
func (plugin *Plugin) getAPIServerURL() string {
	urls, _ := plugin.GetOption(common.OptAPIServerURL).(string)
	if urls == "" {
		urls = defaultAPIServerURL
	}

	return urls
}

// GetSpecPath returns the Docker plugin spec path.
func (plugin *Plugin) getSpecPath() string {
	return os.Getenv("programdata") + pluginSpecPath
}
