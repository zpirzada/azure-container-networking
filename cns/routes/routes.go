// Copyright 2017 Microsoft. All rights reserved.
// MIT License

package routes

import (
	"github.com/Azure/azure-container-networking/log"
)

// Route describes a single route in the routing table.
type Route struct {
	destination string
	mask        string
	gateway     string
	metric      string
	ifaceIndex  int
}

// RoutingTable describes the routing table on the node.
type RoutingTable struct {
	Routes []Route
}

// GetRoutingTable retireves routing table in the node.
func (rt *RoutingTable) GetRoutingTable() error {
	routes, err := getRoutes()
	if err == nil {
		rt.Routes = routes
	}

	return err
}

// RestoreRoutingTable pushes the saved route.
func (rt *RoutingTable) RestoreRoutingTable() error {
	if rt.Routes == nil {
		log.Printf("[Azure CNS] Nothing available in routing table to push")
		return nil
	}

	return putRoutes(rt.Routes)
}
