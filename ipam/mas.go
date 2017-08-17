// Copyright 2017 Microsoft. All rights reserved.
// MIT License

package ipam

import (
	"encoding/json"
	"net"
	"net/http"
	"time"

	"github.com/Azure/azure-container-networking/common"
	"github.com/Azure/azure-container-networking/log"
)

const (
	// Host URL to query.
	masQueryUrl = "http://169.254.169.254:6642/ListNetwork"

	// Minimum time interval between consecutive queries.
	masQueryInterval = 10 * time.Second
)

// Microsoft Azure Stack IPAM configuration source.
type masSource struct {
	name          string
	sink          addressConfigSink
	queryUrl      string
	queryInterval time.Duration
	lastRefresh   time.Time
}

// MAS host agent JSON object format.
type jsonObject struct {
	Isolation string
	IPs       []struct {
		IP              string
		IsolationId     string
		Mask            string
		DefaultGateways []string
		DnsServers      []string
	}
}

// Creates the MAS source.
func newMasSource(options map[string]interface{}) (*masSource, error) {
	queryUrl, _ := options[common.OptIpamQueryUrl].(string)
	if queryUrl == "" {
		queryUrl = masQueryUrl
	}

	i, _ := options[common.OptIpamQueryInterval].(int)
	queryInterval := time.Duration(i) * time.Second
	if queryInterval == 0 {
		queryInterval = masQueryInterval
	}

	return &masSource{
		name:          "MAS",
		queryUrl:      queryUrl,
		queryInterval: queryInterval,
	}, nil
}

// Starts the MAS source.
func (s *masSource) start(sink addressConfigSink) error {
	s.sink = sink
	return nil
}

// Stops the MAS source.
func (s *masSource) stop() {
	s.sink = nil
	return
}

// Refreshes configuration.
func (s *masSource) refresh() error {

	// Refresh only if enough time has passed since the last query.
	if time.Since(s.lastRefresh) < s.queryInterval {
		return nil
	}
	s.lastRefresh = time.Now()

	// Configure the local default address space.
	local, err := s.sink.newAddressSpace(LocalDefaultAddressSpaceId, LocalScope)
	if err != nil {
		return err
	}

	// Fetch configuration.
	resp, err := http.Get(s.queryUrl)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	// Decode JSON object.
	var obj jsonObject
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&obj)
	if err != nil {
		return err
	}

	// Add the IP addresses to the local address space.
	for _, v := range obj.IPs {
		address := net.ParseIP(v.IP)
		subnet := net.IPNet{
			IP:   net.ParseIP(v.IP),
			Mask: net.IPMask(net.ParseIP(v.Mask)),
		}

		ap, err := local.newAddressPool("eth0", 0, &subnet)
		if err != nil {
			log.Printf("[ipam] Failed to create pool:%v err:%v.", subnet, err)
			continue
		}

		_, err = ap.newAddressRecord(&address)
		if err != nil {
			log.Printf("[ipam] Failed to create address:%v err:%v.", address, err)
			continue
		}
	}

	// Set the local address space as active.
	s.sink.setAddressSpace(local)

	return nil
}
