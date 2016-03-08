// Copyright Microsoft Corp.
// All rights reserved.

package core

import (
	"fmt"
	"net"
	"net/http"
	"os/exec"
)

func printHostInterfaces() {
	hostInterfaces, err := net.Interfaces()
	if err != nil {
		fmt.Println("Azure Driver: Got error while retrieving interfaces")
	} else {
		fmt.Println("Azure Driver: Found following Interfaces in default name space")
		for _, hostInterface := range hostInterfaces {

			addresses, ok := hostInterface.Addrs()
			if ok == nil && len(addresses) > 0 {
				fmt.Println("\t", hostInterface.Name, hostInterface.Index, hostInterface.Flags, hostInterface.HardwareAddr, hostInterface.Flags, hostInterface.MTU, addresses[0].String())
			} else {
				fmt.Println("\t", hostInterface.Name, hostInterface.Index, hostInterface.Flags, hostInterface.HardwareAddr, hostInterface.Flags, hostInterface.MTU)
			}
		}
	}
}

func router(w http.ResponseWriter, req *http.Request) {
	fmt.Println("Handler invoked")

	switch req.Method {
	case "GET":
		fmt.Println("receiver GET request", req.URL.Path)
	case "POST":
		fmt.Println("receiver POST request", req.URL.Path)
		switch req.URL.Path {
		case "/Plugin.Activate":
			fmt.Println("/Plugin.Activate received")
		}
	default:
		fmt.Println("receiver unexpected request", req.Method, "->", req.URL.Path)
	}
}

// ExecuteShellCommand executes a shell command
func ExecuteShellCommand(command string) error {
	fmt.Println("going to execute: " + command)
	cmd := exec.Command("sh", "-c", command)
	err := cmd.Start()
	if err != nil {
		return err
	}
	return cmd.Wait()
}
