// Copyright 2018 Microsoft. All rights reserved.
// MIT License

package telemetry

import (
	"fmt"
	"net"
	"os"
	"os/exec"
)

const (
	fdTemplate    = "/tmp/%s.sock"
	PidFile       = "tmp/azuretelemetry.pid"
	MetadatatFile = "/tmp/azuremetadata.json"
)

// Dial - try to connect to/create a socket with 'name'
func (tb *TelemetryBuffer) Dial(name string) (err error) {
	conn, err := net.Dial("unix", fmt.Sprintf(fdTemplate, name))
	if err == nil {
		tb.client = conn
	}

	return err
}

// Listen - try to create and listen on socket with 'name'
func (tb *TelemetryBuffer) Listen(name string) (err error) {
	conn, err := net.Listen("unix", fmt.Sprintf(fdTemplate, name))
	if err == nil {
		tb.listener = conn
	}

	return err
}

// cleanup - manually remove socket
func (tb *TelemetryBuffer) Cleanup(name string) error {
	return os.Remove(fmt.Sprintf(fdTemplate, name))
}

func checkIfSockExists() bool {
	if _, err := os.Stat(fmt.Sprintf(fdTemplate, FdName)); !os.IsNotExist(err) {
		return true
	}

	return false
}

func startTelemetryManager(name string) (int, error) {
	cmd := fmt.Sprintf("/opt/cni/bin/%s", name)
	startCmd := exec.Command("sh", "-c", cmd)
	if err := startCmd.Start(); err != nil {
		return -1, err
	}

	return startCmd.Process.Pid, nil
}
