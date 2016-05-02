// Copyright Microsoft Corp.
// All rights reserved.

package common

const (
	// Libnetwork remote plugin paths
	activatePath = "/Plugin.Activate"
)

//
// Libnetwork remote plugin API
//

// Error response sent by plugin when a request was decoded but failed.
type errorResponse struct {
	Err string
}

// Response sent by plugin for activation.
type activateResponse struct {
	Implements []string
}
