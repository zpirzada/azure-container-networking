// Copyright Microsoft Corp.
// All rights reserved.

package ipam

import (
	"encoding/xml"
	"net"
	"net/http"
	"time"
)

const (
	// Host URL to query.
	azureQueryUrl = "http://168.63.129.16/machine/plugins?comp=nmagent&type=getinterfaceinfov1"

	// Minimum delay between consecutive polls.
	azureDefaultMinPollPeriod = 30 * time.Second
)

// Microsoft Azure IPAM configuration source.
type azureSource struct {
	name          string
	sink          configSink
	lastRefresh   time.Time
	minPollPeriod time.Duration
}

// Azure host agent XML document format.
type xmlDocument struct {
	XMLName   xml.Name `xml:"Interfaces"`
	Interface []struct {
		XMLName    xml.Name `xml:"Interface"`
		MacAddress string   `xml:"MacAddress,attr"`
		IsPrimary  bool     `xml:"IsPrimary,attr"`

		IPSubnet []struct {
			XMLName xml.Name `xml:"IPSubnet"`
			Prefix  string   `xml:"Prefix,attr"`

			IPAddress []struct {
				XMLName   xml.Name `xml:"IPAddress"`
				Address   string   `xml:"Address,attr"`
				IsPrimary bool     `xml:"IsPrimary,attr"`
			}
		}
	}
}

// Creates the Azure source.
func newAzureSource(sink configSink) (*azureSource, error) {
	return &azureSource{
		name:          "Azure",
		sink:          sink,
		minPollPeriod: azureDefaultMinPollPeriod,
	}, nil
}

// Starts the Azure source.
func (s *azureSource) start() error {
	return nil
}

// Stops the Azure source.
func (s *azureSource) stop() {
	return
}

// Refreshes configuration.
func (s *azureSource) refresh() error {

	// Refresh only if enough time has passed since the last poll.
	if time.Since(s.lastRefresh) < s.minPollPeriod {
		return nil
	}
	s.lastRefresh = time.Now()

	// Query the list of local interfaces.
	interfaces, err := net.Interfaces()
	if err != nil {
		return err
	}

	// Configure the local default address space.
	local, err := newAddressSpace(localDefaultAddressSpaceId, localScope)
	if err != nil {
		return err
	}

	// Fetch configuration.
	resp, err := http.Get(azureQueryUrl)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	// Decode XML document.
	var doc xmlDocument
	decoder := xml.NewDecoder(resp.Body)
	err = decoder.Decode(&doc)
	if err != nil {
		return err
	}

	// For each interface...
	for _, i := range doc.Interface {
		// Find the interface with the matching MacAddress.
		ifName := ""
		for _, iface := range interfaces {
			if iface.HardwareAddr.String() == i.MacAddress {
				ifName = iface.Name
				break
			}
		}

		// Skip if interface is not found.
		if ifName == "" {
			continue
		}

		// For each subnet on the interface...
		for _, s := range i.IPSubnet {
			_, subnet, err := net.ParseCIDR(s.Prefix)
			if err != nil {
				return err
			}

			ap, err := local.newAddressPool(ifName, subnet)
			if err != nil && err != errAddressExists {
				return err
			}

			// For each address in the subnet...
			for _, a := range s.IPAddress {
				address := net.ParseIP(a.Address)

				_, err = ap.newAddressRecord(&address)
				if err != nil {
					return err
				}
			}
		}
	}

	// Set the local address space as active.
	s.sink.setAddressSpace(local)

	return nil
}
