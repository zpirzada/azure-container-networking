// Copyright Microsoft Corp.
// All rights reserved.

package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

    "github.com/sharmasushant/penguin/network"
	ipamNull "github.com/sharmasushant/penguin/ipam/null"
)

var Version string = "V0"
var ipamVersion string = "V0"

func main() {
	arguments := os.Args
	if(len(arguments)>1 && os.Args[1] == "startipam"){
		initNullIpam()
	}else{
		initNetworkDriver()
	}

}

func initNetworkDriver(){
	// create a server on which we will listen for incoming calls from libnetwork
	os.MkdirAll("/run/docker/plugins", 0660)
	driverListener, err := net.Listen("unix", "/run/docker/plugins/penguin.sock")
	if err != nil {
		fmt.Println("Error setting up driver listener: ", err)
	}
	defer driverListener.Close()

	// For now, driver can shutdown on two conditions
	//    a. If some unhandled exception happens in the driver
	//    b. If we receive explicit signal
	// To receive explicit os signals, create a channel that can receive os signals.
	osSignalChannel := make(chan os.Signal, 1)

	// Relay incoming signals to channel.
	// If no signals are provided, all incoming signals will be relayed to channel.
	// Otherwise, just the provided signals will.
	signal.Notify(osSignalChannel, os.Interrupt, os.Kill, syscall.SIGTERM)

	// create a channel that can receive errors from driver
	// any unhandled error from driver will be sent to this channel
	errorChan := make(chan error, 1)
	driver, err := network.NewPlugin(Version)
	go func() {
		errorChan <- driver.StartListening(driverListener)
	}()

	fmt.Printf("Penguin is ready..\n")

	// select can be used to wait on multiple channels in parallel
	select {
	case sig := <-osSignalChannel:
		fmt.Println("\nCaught signal <" + sig.String() + "> shutting down..")
	case err := <-errorChan:
		if err != nil {
			fmt.Println("\nDriver received an unhandled error.. ", err)
			driverListener.Close()
			os.Exit(1)
		}
	}
}

func initNullIpam(){
	// create a server on which we will listen for incoming calls from libnetwork
	os.MkdirAll("/run/docker/plugins", 0660)
	driverListener, err := net.Listen("unix", "/run/docker/plugins/nullipam.sock")
	if err != nil {
		fmt.Println("Error setting up null ipam listener socket: ", err)
	}
	defer driverListener.Close()

	// For now, driver can shutdown on two conditions
	//    a. If some unhandled exception happens in the driver
	//    b. If we receive explicit signal
	// To receive explicit os signals, create a channel that can receive os signals.
	osSignalChannel := make(chan os.Signal, 1)

	// Relay incoming signals to channel.
	// If no signals are provided, all incoming signals will be relayed to channel.
	// Otherwise, just the provided signals will.
	signal.Notify(osSignalChannel, os.Interrupt, os.Kill, syscall.SIGTERM)

	// create a channel that can receive errors from driver
	// any unhandled error from driver will be sent to this channel
	errorChan := make(chan error, 1)
	driver, err := ipamNull.NewPlugin(ipamVersion)
	go func() {
		errorChan <- driver.StartListening(driverListener)
	}()

	fmt.Printf("Null IPAM is ready..\n")

	// select can be used to wait on multiple channels in parallel
	select {
	case sig := <-osSignalChannel:
		fmt.Println("\nCaught signal <" + sig.String() + "> shutting down..")
	case err := <-errorChan:
		if err != nil {
			fmt.Println("\nDriver received an unhandled error.. ", err)
			driverListener.Close()
			os.Exit(1)
		}
	}
}
