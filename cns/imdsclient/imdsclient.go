// Copyright 2017 Microsoft. All rights reserved.
// MIT License

package imdsclient

import (
	"fmt"
	"strings"
	"encoding/xml"
	"net/http"

	"github.com/Azure/azure-container-networking/log"
)

// GetPrimaryInterfaceInfoFromHost retrieves subnet and gateway of primary NIC from Host.
func (imdsClient *ImdsClient) GetPrimaryInterfaceInfoFromHost() (*InterfaceInfo, error) {	
	log.Printf("[Azure CNS] GetPrimaryInterfaceInfoFromHost")
	interfaceInfo := &InterfaceInfo{}
	resp, err := http.Get(hostQueryURL)
	if(err != nil){
		return nil, err
	}

	log.Printf("[Azure CNS] Response received from NMAgent: %v", resp.Body)
	
	var doc xmlDocument
	decoder := xml.NewDecoder(resp.Body)
	err = decoder.Decode(&doc)
	if err != nil {
		return nil, err
	}
	
	foundInterface := false

	// For each interface.
	for _, i := range doc.Interface {		
		// Find primary Interface.
		if i.IsPrimary {
			// Get the first subnet.
			for _, s := range i.IPSubnet {
				interfaceInfo.Subnet = s.Prefix				
				malformedSubnetError := fmt.Errorf("Malformed subnet received from host %s", s.Prefix)

				st := strings.Split(s.Prefix, "/")
				if(len(st) != 2){
					return nil, malformedSubnetError
				}
				
				ip := strings.Split(st[0], ".")	
				if(len(ip) != 4){
					return nil, malformedSubnetError
				}

				interfaceInfo.Gateway = fmt.Sprintf("%s.%s.%s.1", ip[0], ip[1], ip[2])
				foundInterface = true
				break;
			}
			break;
		}
	}	
	var er error
	er = nil
	if (!foundInterface) {
		er = fmt.Errorf("Unable to find primary NIC")
	}
	return interfaceInfo, er 
}