// Copyright Microsoft Corp.
// All rights reserved.

package ebtables

import (
	"fmt"
	"io/ioutil"
	"net"
	"os/exec"
	"strings"

	"github.com/Azure/azure-container-networking/log"
)

const (
	// Ebtables actions.
	Append = "-A"
	Delete = "-D"
)

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

// SetSnatForInterface sets a MAC SNAT rule for an interface.
func SetSnatForInterface(interfaceName string, macAddress net.HardwareAddr, action string) error {
	command := fmt.Sprintf(
		"ebtables -t nat %s POSTROUTING -o %s -j snat --to-src %s --snat-arp",
		action, interfaceName, macAddress.String())

	return executeShellCommand(command)
}

// SetDnatForArpReplies sets a MAC DNAT rule for ARP replies received on an interface.
func SetDnatForArpReplies(interfaceName string, action string) error {
	command := fmt.Sprintf(
		"ebtables -t nat %s PREROUTING -p ARP -i %s -j dnat --to-dst ff:ff:ff:ff:ff:ff",
		action, interfaceName)

	return executeShellCommand(command)
}

// SetDnatForIPAddress sets a MAC DNAT rule for an IP address.
func SetDnatForIPAddress(ipAddress net.IP, macAddress net.HardwareAddr, action string) error {
	command := fmt.Sprintf(
		"ebtables -t nat %s PREROUTING -p IPv4 --ip-dst %s -j dnat --to-dst %s",
		action, ipAddress.String(), macAddress.String())

	return executeShellCommand(command)
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
