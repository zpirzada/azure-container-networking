// Node network service grpc client
package nnsgrpcclient

import (
	"context"
	"fmt"
	"github.com/Azure/azure-container-networking/log"
	nns "github.com/Azure/azure-container-networking/proto/nodenetworkservice/3.302.0.744"
	"google.golang.org/grpc"
	"time"
)

const (
	nnsPort = "6668"
	apiTimeout = 5 * time.Minute // recommended timeout from node service
	connectionTimeout = 2 * time.Minute
)

// client to invoke Node network service APIs using grpc
type NnsGrpcClient struct {
}

// Add container to the network. Container Id is appended to the podName
func (c *NnsGrpcClient) AddContainerNetworking(
	ctx context.Context,
	podName, nwNamespace string) (error, *nns.ConfigureContainerNetworkingResponse)   {

	return  configureContainerNetworking(ctx, nns.RequestType_Setup, podName, nwNamespace)
}

// Add container to the network. Container Id is appended to the podName
func (c *NnsGrpcClient) DeleteContainerNetworking(
	ctx context.Context,
	podName, nwNamespace string) (error, *nns.ConfigureContainerNetworkingResponse)   {

	return  configureContainerNetworking(ctx, nns.RequestType_Teardown, podName, nwNamespace)
}

func configureContainerNetworking (
	ctx context.Context,
	reqtype nns.RequestType,
	podName, nwNamespace string) (error, *nns.ConfigureContainerNetworkingResponse) {

	client, conn, err := newNnsGrpcClient(ctx)
	if err != nil {
		return err, nil
	}

	defer func() {
		if err := conn.Close(); err != nil {
            log.Printf("grpc connection close for container %s failed with : %s", podName, err)
		}
	}()

	req := &nns.ConfigureContainerNetworkingRequest{
		RequestType: reqtype,
		ContainerId: podName,
		NetworkNamespaceId: nwNamespace,
	}

	localCtx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	res, err := client.ConfigureContainerNetworking(localCtx, req)
	if err != nil {
		log.Printf("ConfigureContainerNetworking for container %s for nw namespace %s failed with error: %s \n",
			podName, nwNamespace, err)
	} else {
		log.Printf("ConfigureContainerNetworking for container %s for nw namespace %s succeeded with result: %v \n",
			podName, nwNamespace, res)
	}

    return err, res
}

func newNnsGrpcClient (ctx context.Context) (nns.NodeNetworkServiceClient, *grpc.ClientConn,  error)  {

	localCtx, cancel := context.WithTimeout(ctx, connectionTimeout)
	defer cancel()

    nnsEndpoint := fmt.Sprintf(":%s", nnsPort)
	conn, err := grpc.DialContext(
		localCtx, nnsEndpoint, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return nil, nil, fmt.Errorf("connection to nns failed at endpoint %s with error: %s", nnsEndpoint, err)
	}

	return nns.NewNodeNetworkServiceClient(conn), conn, nil
}