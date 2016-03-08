// Copyright Microsoft Corp.
// All rights reserved.

package ebtables

import (
	"fmt"
	"os/exec"
)

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
	fmt.Println("going to execute: " + command)
	cmd := exec.Command("sh", "-c", command)
	err := cmd.Start()
	if err != nil {
		return err
	}
	return cmd.Wait()
}
