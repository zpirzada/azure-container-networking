package main

import (
	"time"

	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/telemetry"
)

const (
	reportToHostInterval = 120
)

func main() {
	var tb *telemetry.TelemetryBuffer
	var err error

	log.Printf("TelemetryBuffer process started")
	for {
		tb, err = telemetry.NewTelemetryBuffer(true)
		if err == nil {
			log.Printf("Server started")
			break
		}
		log.Printf("[Telemetry] Failed to establish telemetry buffer connection.")
		time.Sleep(time.Minute * 1)
	}

	tb.Start(reportToHostInterval)
	log.Printf("TelemetryBuffer process exiting")
}
