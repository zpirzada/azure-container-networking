package cni

import (
	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/platform"
)

func removeLockFileAfterReboot(plugin *Plugin) {
	if lockFileModTime, err := plugin.Store.GetLockFileModificationTime(); err == nil {
		rebootTime, err := platform.GetLastRebootTime()
		log.Printf("[cni] reboot time %v storeLockFile mod time %v", rebootTime, lockFileModTime)
		if err == nil && rebootTime.After(lockFileModTime) {
			log.Printf("[cni] Detected Reboot")

			if err := plugin.Store.Unlock(true); err != nil {
				log.Printf("[cni] Failed to force unlock store due to error %v", err)
			} else {
				log.Printf("[cni] Force unlocked the store successfully")
			}
		}
	}
}
