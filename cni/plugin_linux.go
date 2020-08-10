package cni

import (
	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/platform"
)

func removeLockFileAfterReboot(plugin *Plugin) {
	rebootTime, _ := platform.GetLastRebootTime()
	log.Printf("[cni] reboot time %v", rebootTime)
}
