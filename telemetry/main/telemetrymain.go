package main

import (
	"fmt"
	"time"

	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/telemetry"
)

const (
	reportToHostInterval = 10000
	azuretelemetry       = "azuretelemetry"
)

func main() {
	var tb *telemetry.TelemetryBuffer
	var err error

	log.SetName(azuretelemetry)
	log.SetLevel(log.LevelInfo)
	err = log.SetTarget(log.TargetLogfile)
	if err != nil {
		fmt.Printf("log settarget failed")
	}

	log.Printf("[Telemetry] TelemetryBuffer process started")
	for {
		tb = telemetry.NewTelemetryBuffer()
		err = tb.StartServer()
		if err == nil || tb.FdExists {
			log.Printf("[Telemetry] Server started")
			break
		}

		tb.Cleanup(telemetry.FdName)

		log.Printf("[Telemetry] Failed to establish telemetry buffer connection.")
		time.Sleep(time.Minute * 1)
	}

	tb.BufferAndPushData(reportToHostInterval)
	log.Printf("TelemetryBuffer process exiting")
}
