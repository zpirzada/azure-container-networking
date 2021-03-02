package nnsgrpcclient

import (
	"context"
	"fmt"
	"github.com/Azure/azure-container-networking/test/nnsmockserver"
	"os"
	"strings"
	"testing"
	"time"
)

var mockserver *nnsmockserver.NnsMockServer

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	teardown()
	os.Exit(code)
}

func setup() {
	mockserver  = nnsmockserver.NewNnsMockServer()
	go  mockserver.StartGrpcServer(nnsPort)
	fmt.Printf("mock nns server started\n")
}

func teardown() {
	mockserver.StopGrpcServer()
	fmt.Printf("mock nns server stopped\n")
}

// CNI ADD to add container to network
func TestAddContainerNetworking(t *testing.T) {

	client := &NnsGrpcClient{}
	if err, _ := client.AddContainerNetworking(
		 context.Background(),
		"sf_8e9961f4-5b4f-4b3c-a9ae-c3294b0d9681",
		"testnwspace"); err != nil {
       t.Fatalf("TestAddContainerNetworking failed: %v", err)
	}

	fmt.Printf("TestAddContainerNetworking success\n")
}

// CNI DEL to delete container from network
func TestDeleteContainerNetworking(t *testing.T) {

	client := &NnsGrpcClient{}
	if err, _ := client.DeleteContainerNetworking(
		context.Background(),
		"sf_8e9961f4-5b4f-4b3c-a9ae-c3294b0d9681",
		"testnwspace"); err != nil {
		t.Fatalf("TestSetupContainer: %v", err)
	}

	fmt.Printf("TestDeleteContainerNetworking: success\n")
}

// CNI ADD to add container to network - failure case
func TestAddContainerNetworkingFailure(t *testing.T) {

	client := &NnsGrpcClient{}

	var err error
	if err, _ = client.AddContainerNetworking(context.Background(), "testpod", "testnwspace"); err == nil {
		t.Fatalf("TestAddContainerNetworkingFailure failed. Expected error but none returned\n")
	}

	if !strings.Contains(err.Error(), "Setup") {
		t.Fatalf("TestAddContainerNetworkingFailure failed. Error should have contained Setup. %v \n", err)
	}

	fmt.Printf("TestAddContainerNetworkingFailure success\n")
}

// CNI DEL to add container to network - failure case
func TestDeleteContainerNetworkingFailure(t *testing.T) {

	client := &NnsGrpcClient{}

	var err error
	if err, _ = client.DeleteContainerNetworking(context.Background(), "testpod", "testnwspace"); err == nil {
		t.Fatalf("TestDeleteContainerNetworkingFailure failed. Expected error but none returned\n")
	}

	if !strings.Contains(err.Error(), "Teardown") {
		t.Fatalf("TestDeleteContainerNetworkingFailure failed. Error should have contained Teardown. %v \n", err)
	}

	fmt.Printf("TestDeleteContainerNetworkingFailure success\n")
}

// CNI ADD to add container to network - grpc server was down temporarily and client reconnects well
func TestAddContainerNetworkingGrpcServerDown(t *testing.T) {

	// shutdown server to simulate connection error
	teardown()

	// bring up the server again after 25 seconds
	go func() {
		time.Sleep(25 * time.Second)
		setup()
	}()

	client := &NnsGrpcClient{}

	var err error
	if err, _ = client.AddContainerNetworking(
		context.Background(),"sf_8e9961f4-5b4f-4b3c-a9ae-c3294b0d9681", "testnwspace"); err != nil {

		t.Fatalf("TestAddContainerNetworkingGrpcServerDown failed. %s\n", err)
	}

	fmt.Printf("TestAddContainerNetworkingGrpcServerDown success\n")
}