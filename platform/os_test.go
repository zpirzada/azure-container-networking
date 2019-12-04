package platform

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	exitCode := m.Run()
	os.Exit(exitCode)
}

func TestGetLastRebootTime(t *testing.T) {
	_, err := GetLastRebootTime()
	if err != nil {
		t.Errorf("GetLastRebootTime failed :%v", err)
	}
}

func TestGetOSDetails(t *testing.T) {
	_, err := GetOSDetails()
	if err != nil {
		t.Errorf("GetOSDetails failed :%v", err)
	}
}

func TestGetProcessNameByID(t *testing.T) {
	pName, err := GetProcessNameByID(strconv.Itoa(os.Getpid()))
	if err != nil {
		t.Errorf("GetProcessNameByID failed: %v", err)
	}

	if !strings.Contains(pName, "platform.test") {
		t.Errorf("Incorrect process name:%v\n", pName)
	}
}
