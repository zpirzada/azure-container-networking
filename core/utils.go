// Copyright Microsoft Corp.
// All rights reserved.

package core

import (
	"io/ioutil"
	"net"
	"os/exec"

	"github.com/Azure/Aqua/log"
)

// LogPlatformInfo logs platform version information.
func logPlatformInfo() {
	info, err := ioutil.ReadFile("/proc/version")
	if err == nil {
		log.Printf("[core] Running on %v", string(info))
	} else {
		log.Printf("[core] Failed to detect platform, err:%v", err)
	}
}

// LogNetworkInterfaces logs the host's network interfaces in the default namespace.
func logNetworkInterfaces() {
	interfaces, err := net.Interfaces()
	if err != nil {
		log.Printf("[core] Failed to query network interfaces, err:%v", err)
		return
	}

	for _, iface := range interfaces {
		addrs, _ := iface.Addrs()
		log.Printf("[core] Network interface: %+v with IP addresses: %+v", iface, addrs);
	}
}

// ExecuteShellCommand executes a shell command.
func ExecuteShellCommand(command string) error {
	log.Debugf("[core] %s", command)
	cmd := exec.Command("sh", "-c", command)
	err := cmd.Start()
	if err != nil {
		return err
	}
	return cmd.Wait()
}
