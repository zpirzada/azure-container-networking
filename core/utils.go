// Copyright Microsoft Corp.
// All rights reserved.

package core

import (
	"fmt"
	"log"
	"net"
	"net/http"
)

func LogRequest(tag string, requestName string, err error) {
    if err == nil {
        log.Printf("%s: Received %s request.", tag, requestName)
	} else {
        log.Printf("%s: Failed to decode %s request %s.", tag, requestName, err.Error())
	}
}

func LogResponse(tag string, responseName string, response interface{}, err error) {
    if err == nil {
        log.Printf("%s: Sent %s response %+v.", tag, responseName, response)
	} else {
        log.Printf("%s: Failed to encode %s response %+v %s.", tag, responseName, response, err.Error())
	}
}

func LogEvent(tag string, event string) {
    log.Printf("%s: %s", tag, event)
}

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
