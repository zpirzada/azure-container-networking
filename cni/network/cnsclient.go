package network

import (
	"context"

	"github.com/Azure/azure-container-networking/cns"
)

type cnsclient interface {
	RequestIPAddress(ctx context.Context, ipconfig cns.IPConfigRequest) (*cns.IPConfigResponse, error)
	RequestIPs(ctx context.Context, ipconfig cns.IPConfigRequest) (*cns.IPConfigsResponse, error)
	ReleaseIPs(ctx context.Context, ipconfig cns.IPConfigRequest) error
	GetNetworkConfiguration(ctx context.Context, orchestratorContext []byte) (*cns.GetNetworkContainerResponse, error)
}
