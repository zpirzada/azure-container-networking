// Copyright 2017 Microsoft. All rights reserved.
// MIT License

package netlink

import (
	"fmt"
	"testing"
)

const ifname string = "nltest"

// Tests basic netlink messaging via echo.
func TestEcho(t *testing.T) {
	fmt.Println("Test: Echo")

	err := Echo("this is a test")

	if err != nil {
		t.Errorf("Echo failed: %+v", err)
	}
}

// Tests creating a new network interface.
func TestAddLink(t *testing.T) {
	fmt.Println("Test: AddLink")

	err := AddLink(ifname, "bridge")

	if err != nil {
		t.Errorf("AddLink failed: %+v", err)
	}
}

// Tests setting the operational state of a network interface.
func TestSetLinkState(t *testing.T) {
	fmt.Println("Test: SetLinkState")

	err := SetLinkState(ifname, true)

	if err != nil {
		t.Errorf("SetLinkState up failed: %+v", err)
	}

	err = SetLinkState(ifname, false)

	if err != nil {
		t.Errorf("SetLinkState down failed: %+v", err)
	}
}

// Tests deleting a network interface.
func TestDeleteLink(t *testing.T) {
	fmt.Println("Test: DeleteLink")

	err := DeleteLink(ifname)

	if err != nil {
		t.Errorf("DeleteLink failed: %+v", err)
	}
}
