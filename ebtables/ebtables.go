// Copyright Microsoft Corp.
// All rights reserved.

package ebtables

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"

	"github.com/Azure/azure-container-networking/log"
)

// Init initializes the ebtables module.
func init() {
	installEbtables()
}

// InstallEbtables installs the ebtables package.
func installEbtables() {
	version, _ := ioutil.ReadFile("/proc/version")
	os := strings.ToLower(string(version))

	if strings.Contains(os, "ubuntu") {
		executeShellCommand("apt-get install ebtables")
	} else if strings.Contains(os, "redhat") {
		executeShellCommand("yum install ebtables")
	} else {
		log.Printf("Unable to detect OS platform. Please make sure the ebtables package is installed.")
	}
}

// SetupSnatForOutgoingPackets sets up snat
func SetupSnatForOutgoingPackets(interfaceName string, snatAddress string) error {
	command := fmt.Sprintf("ebtables -t nat -A POSTROUTING -o %s -j snat --to-source %s --snat-arp", interfaceName, snatAddress)
	err := executeShellCommand(command)
	if err != nil {
		return err
	}
	return nil
}

// CleanupSnatForOutgoingPackets cleans up snat
func CleanupSnatForOutgoingPackets(interfaceName string, snatAddress string) error {
	command := fmt.Sprintf("ebtables -t nat -D POSTROUTING -o %s -j snat --to-source %s --snat-arp", interfaceName, snatAddress)
	err := executeShellCommand(command)
	if err != nil {
		return err
	}
	return nil
}

// SetupDnatForArpReplies sets up dnat
func SetupDnatForArpReplies(interfaceName string) error {
	command := fmt.Sprintf("ebtables -t nat -A PREROUTING -i %s -p arp -j dnat --to-destination ff:ff:ff:ff:ff:ff", interfaceName)
	err := executeShellCommand(command)
	if err != nil {
		return err
	}
	return nil
}

// CleanupDnatForArpReplies cleans up dnat
func CleanupDnatForArpReplies(interfaceName string) error {
	command := fmt.Sprintf("ebtables -t nat -D PREROUTING -i %s -p arp -j dnat --to-destination ff:ff:ff:ff:ff:ff", interfaceName)
	err := executeShellCommand(command)
	if err != nil {
		return err
	}
	return nil
}

// SetupDnatBasedOnIPV4Address sets up dnat
func SetupDnatBasedOnIPV4Address(ipv4Address string, macAddress string) error {
	command := fmt.Sprintf("ebtables -t nat -A PREROUTING -p IPv4  --ip-dst %s -j dnat --to-dst %s --dnat-target ACCEPT", ipv4Address, macAddress)
	err := executeShellCommand(command)
	if err != nil {
		return err
	}
	return nil
}

// RemoveDnatBasedOnIPV4Address cleans up dnat
func RemoveDnatBasedOnIPV4Address(ipv4Address string, macAddress string) error {
	command := fmt.Sprintf("ebtables -t nat -D PREROUTING -p IPv4  --ip-dst %s -j dnat --to-dst %s --dnat-target ACCEPT", ipv4Address, macAddress)
	err := executeShellCommand(command)
	if err != nil {
		return err
	}
	return nil
}

func executeShellCommand(command string) error {
	log.Debugf("[ebtables] %s", command)
	cmd := exec.Command("sh", "-c", command)
	err := cmd.Start()
	if err != nil {
		return err
	}
	return cmd.Wait()
}
