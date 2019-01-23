// Copyright 2018 Microsoft. All rights reserved.
// MIT License

package telemetry

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/Microsoft/go-winio"
)

const (
	fdTemplate    = "\\\\.\\pipe\\%s"
	PidFile       = "azuretelemetry.pid"
	MetadatatFile = "azuremetadata.json"
)

// Dial - try to connect to a named pipe with 'name'
func (tb *TelemetryBuffer) Dial(name string) (err error) {
	conn, err := winio.DialPipe(fmt.Sprintf(fdTemplate, name), nil)
	if err == nil {
		tb.client = conn
	}

	return err
}

// Listen - try to create and listen on named pipe with 'name'
func (tb *TelemetryBuffer) Listen(name string) (err error) {
	listener, err := winio.ListenPipe(fmt.Sprintf(fdTemplate, name), nil)
	if err == nil {
		tb.listener = listener
	}

	return err
}

// cleanup - stub
func (tb *TelemetryBuffer) Cleanup(name string) error {
	return nil
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
