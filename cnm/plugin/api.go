// Copyright Microsoft Corp.
// All rights reserved.

package plugin

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

// Request sent by libnetwork for activation.
type activateRequest struct {
}

// Response sent by plugin for activation.
type activateResponse struct {
	Implements []string
}
