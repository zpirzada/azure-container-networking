// Copyright 2018 Microsoft. All rights reserved.
// MIT License

package telemetry

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Azure/azure-container-networking/log"
)

// FdName - file descriptor name
// Delimiter - delimiter for socket reads/writes
// HostNetAgentURL - host net agent url of type payload
// DefaultDncReportsSize - default DNC report slice size
// DefaultCniReportsSize - default CNI report slice size
// DefaultNpmReportsSize - default NPM report slice size
// DefaultInterval - default interval for sending payload to host
const (
	FdName                  = "azure-telemetry"
	Delimiter               = '\n'
	HostNetAgentURL         = "http://169.254.169.254/machine/plugins?comp=netagent&type=payload"
	telemetryMgrProcessName = "azuretelemetrymgr"
	DefaultInterval         = 1 * time.Minute
)

// TelemetryBuffer object
type TelemetryBuffer struct {
	client      net.Conn
	listener    net.Listener
	connections []net.Conn
	payload     Payload
	FdExists    bool
	Connected   bool
	data        chan interface{}
	cancel      chan bool
}

// Payload object holds the different types of reports
type Payload struct {
	DNCReports []DNCReport
	CNIReports []CNIReport
	NPMReports []NPMReport
	CNSReports []CNSReport
}

// NewTelemetryBuffer - create a new TelemetryBuffer
func NewTelemetryBuffer() *TelemetryBuffer {
	var tb TelemetryBuffer
	tb.data = make(chan interface{})
	tb.cancel = make(chan bool, 1)
	tb.connections = make([]net.Conn, 1)
	tb.payload.DNCReports = make([]DNCReport, 0)
	tb.payload.CNIReports = make([]CNIReport, 0)
	tb.payload.NPMReports = make([]NPMReport, 0)
	tb.payload.CNSReports = make([]CNSReport, 0)
	return &tb
}

func (tb *TelemetryBuffer) StartServer() error {
	err := tb.Listen(FdName)
	if err != nil {
		tb.FdExists = strings.Contains(err.Error(), "in use") || strings.Contains(err.Error(), "Access is denied")
		return err
	}

	// Spawn server goroutine to handle incoming connections
	go func() {
		for {
			// Spawn worker goroutines to communicate with client
			conn, err := tb.listener.Accept()
			if err == nil {
				tb.connections = append(tb.connections, conn)
				go func() {
					for {
						reportStr, err := read(conn)
						if err == nil {
							var tmp map[string]interface{}
							json.Unmarshal(reportStr, &tmp)
							if _, ok := tmp["NpmVersion"]; ok {
								var npmReport NPMReport
								json.Unmarshal([]byte(reportStr), &npmReport)
								tb.data <- npmReport
							} else if _, ok := tmp["CniSucceeded"]; ok {
								log.Printf("Got cni report")
								var cniReport CNIReport
								json.Unmarshal([]byte(reportStr), &cniReport)
								tb.data <- cniReport
							} else if _, ok := tmp["Allocations"]; ok {
								var dncReport DNCReport
								json.Unmarshal([]byte(reportStr), &dncReport)
								tb.data <- dncReport
							} else if _, ok := tmp["DncPartitionKey"]; ok {
								var cnsReport CNSReport
								json.Unmarshal([]byte(reportStr), &cnsReport)
								tb.data <- cnsReport
							}
						}
					}
				}()
			}
		}
	}()

	return nil
}

func (tb *TelemetryBuffer) Connect() error {
	err := tb.Dial(FdName)
	if err == nil {
		tb.Connected = true
	} else if tb.FdExists {
		tb.Cleanup(FdName)
	}

	return err
}

// BufferAndPushData - BufferAndPushData running an instance if it isn't already being run elsewhere
func (tb *TelemetryBuffer) BufferAndPushData(intervalms time.Duration) {
	defer tb.close()
	if !tb.FdExists {
		log.Printf("Buffer telemetry data and send it to host")
		if intervalms < DefaultInterval {
			intervalms = DefaultInterval
		}

		interval := time.NewTicker(intervalms).C
		for {
			select {
			case <-interval:
				// Send payload to host and clear cache when sent successfully
				// To-do : if we hit max slice size in payload, write to disk and process the logs on disk on future sends
				log.Printf("send data to host")
				if err := tb.sendToHost(); err == nil {
					tb.payload.reset()
				} else {
					log.Printf("sending to host failed with error %+v", err)
				}
			case report := <-tb.data:
				log.Printf("Got data..buffer it")
				tb.payload.push(report)
			case <-tb.cancel:
				goto EXIT
			}
		}
	} else {
		<-tb.cancel
	}

EXIT:
}

// read - read from the file descriptor
func read(conn net.Conn) (b []byte, err error) {
	b, err = bufio.NewReader(conn).ReadBytes(Delimiter)
	if err == nil {
		b = b[:len(b)-1]
	}

	return
}

// Write - write to the file descriptor
func (tb *TelemetryBuffer) Write(b []byte) (c int, err error) {
	b = append(b, Delimiter)
	w := bufio.NewWriter(tb.client)
	c, err = w.Write(b)
	if err == nil {
		err = w.Flush()
	}

	return
}

// Cancel - signal to tear down telemetry buffer
func (tb *TelemetryBuffer) Cancel() {
	tb.cancel <- true
}

// close - close all connections
func (tb *TelemetryBuffer) close() {
	if tb.client != nil {
		tb.client.Close()
	}

	if tb.listener != nil {
		tb.listener.Close()
	}

	for _, conn := range tb.connections {
		if conn != nil {
			conn.Close()
		}
	}
}

// sendToHost - send payload to host
func (tb *TelemetryBuffer) sendToHost() error {
	httpc := &http.Client{}
	var body bytes.Buffer
	log.Printf("Sending payload %+v", tb.payload)
	json.NewEncoder(&body).Encode(tb.payload)
	resp, err := httpc.Post(HostNetAgentURL, ContentType, &body)
	if err != nil {
		return fmt.Errorf("[Telemetry] HTTP Post returned error %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("[Telemetry] HTTP Post returned statuscode %d", resp.StatusCode)
	}

	return nil
}

// push - push the report (x) to corresponding slice
func (pl *Payload) push(x interface{}) {

	metadata, err := getHostMetadata()
	if err != nil {
		log.Printf("Error getting metadata %v", err)
		return
	}

	err = saveHostMetadata(metadata)
	if err != nil {
		log.Printf("saving host metadata failed with :%v", err)
	}

	switch x.(type) {
	case DNCReport:
		dncReport := x.(DNCReport)
		dncReport.Metadata = metadata
		pl.DNCReports = append(pl.DNCReports, dncReport)
	case CNIReport:
		cniReport := x.(CNIReport)
		metadata.Tags = metadata.Tags + ";" + cniReport.Version
		cniReport.Metadata = metadata
		log.Printf("cni report : %+v", cniReport)
		pl.CNIReports = append(pl.CNIReports, cniReport)
	case NPMReport:
		npmReport := x.(NPMReport)
		npmReport.Metadata = metadata
		pl.NPMReports = append(pl.NPMReports, npmReport)
	case CNSReport:
		cnsReport := x.(CNSReport)
		cnsReport.Metadata = metadata
		pl.CNSReports = append(pl.CNSReports, cnsReport)
	}
}

// reset - reset payload slices
func (pl *Payload) reset() {
	pl.DNCReports = nil
	pl.DNCReports = make([]DNCReport, 0)
	pl.CNIReports = nil
	pl.CNIReports = make([]CNIReport, 0)
	pl.NPMReports = nil
	pl.NPMReports = make([]NPMReport, 0)
	pl.CNSReports = nil
	pl.CNSReports = make([]CNSReport, 0)
}

func saveHostMetadata(metadata Metadata) error {
	dataBytes, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("[Telemetry] marshal data failed with err %+v", err)
	}

	if err = ioutil.WriteFile(MetadatatFile, dataBytes, 0644); err != nil {
		log.Printf("[telemetry] Writing metadata to file failed: %v", err)
	}

	return err
}

// GetHostMetadata - retrieve metadata from host
func getHostMetadata() (Metadata, error) {
	content, err := ioutil.ReadFile(MetadatatFile)
	if err == nil {
		var metadata Metadata
		if err = json.Unmarshal(content, &metadata); err == nil {
			log.Printf("Returning hostmetadata saved in state")
			return metadata, nil
		}
	}

	req, err := http.NewRequest("GET", metadataURL, nil)
	if err != nil {
		return Metadata{}, err
	}

	req.Header.Set("Metadata", "True")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return Metadata{}, err
	}

	defer resp.Body.Close()

	metareport := metadataWrapper{}

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("[Telemetry] Request failed with HTTP error %d", resp.StatusCode)
	} else if resp.Body != nil {
		err = json.NewDecoder(resp.Body).Decode(&metareport)
		if err == nil {
			// // Find Metadata struct in report and try to set values
			// v := reflect.ValueOf(report).Elem().FieldByName("Metadata")
			// log.Printf("populate metadata %v", v)
			// if v.CanSet() {
			// 	v.FieldByName("Location").SetString(metareport.Metadata.Location)
			// 	v.FieldByName("VMName").SetString(metareport.Metadata.VMName)
			// 	v.FieldByName("Offer").SetString(metareport.Metadata.Offer)
			// 	v.FieldByName("OsType").SetString(metareport.Metadata.OsType)
			// 	v.FieldByName("PlacementGroupID").SetString(metareport.Metadata.PlacementGroupID)
			// 	v.FieldByName("PlatformFaultDomain").SetString(metareport.Metadata.PlatformFaultDomain)
			// 	v.FieldByName("PlatformUpdateDomain").SetString(metareport.Metadata.PlatformUpdateDomain)
			// 	v.FieldByName("Publisher").SetString(metareport.Metadata.Publisher)
			// 	v.FieldByName("ResourceGroupName").SetString(metareport.Metadata.ResourceGroupName)
			// 	v.FieldByName("Sku").SetString(metareport.Metadata.Sku)
			// 	v.FieldByName("SubscriptionID").SetString(metareport.Metadata.SubscriptionID)
			// 	v.FieldByName("Tags").SetString(metareport.Metadata.Tags)
			// 	v.FieldByName("OSVersion").SetString(metareport.Metadata.OSVersion)
			// 	v.FieldByName("VMID").SetString(metareport.Metadata.VMID)
			// 	v.FieldByName("VMSize").SetString(metareport.Metadata.VMSize)
			// 	log.Printf("done metadata")
			// } else {
			// 	log.Printf("not able to set")
			// 	err = fmt.Errorf("[Telemetry] Unable to set metadata values")
			// }
		} else {
			err = fmt.Errorf("[Telemetry] Unable to decode response body due to error: %s", err.Error())
		}
	} else {
		err = fmt.Errorf("[Telemetry] Response body is empty")
	}

	return metareport.Metadata, err
}

func StartTelemetryManagerProcess() error {
	content, err := ioutil.ReadFile(PidFile)
	if err == nil {
		pidStr := strings.TrimSpace(string(content))
		log.Printf("telemetry pid %v. check if its running", pidStr)
		pid, _ := strconv.Atoi(pidStr)
		process, err := os.FindProcess(pid)
		if err == nil {
			err := process.Signal(syscall.Signal(0))
			if err == nil && checkIfSockExists() {
				log.Printf("azure telemetry process already running with pid %v", pid)
				return nil
			} else if err == nil {
				process.Kill()
			}
		}
	}

	log.Printf("[telemetry] Starting telemetry manager process")
	pid, err := startTelemetryManager(telemetryMgrProcessName)
	if err != nil {
		log.Printf("Failed to start telemetry manager process :%v", err)
		return err
	}

	log.Printf("[Telemetry] Telemetry manager process started with pid %d", pid)
	time.Sleep(2 * time.Second)

	if err := os.Truncate(PidFile, 0); err != nil {
		log.Printf("[Telemetry] Truncating pid file failed %v", err)
	}

	buf := []byte(strconv.Itoa(pid))

	if err := ioutil.WriteFile(PidFile, buf, 0644); err != nil {
		log.Printf("[telemetry] Writing pid to file failed: %v", err)
		return nil
	}

	log.Printf("saved pid %v", pid)
	return nil
}
