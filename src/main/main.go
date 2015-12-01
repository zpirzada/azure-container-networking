package main

import (
	//"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"dockerdriver"
	//"log/syslog"
)

var Version string = "V0"

func main() {

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
	driver, err := dockerdriver.NewInstance(Version)
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
