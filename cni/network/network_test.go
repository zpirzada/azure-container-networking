package network

import (
	"fmt"
	"net"
	"os"
	"testing"

	"github.com/Azure/azure-container-networking/cni"
	"github.com/Azure/azure-container-networking/cni/api"
	"github.com/Azure/azure-container-networking/common"
	acnnetwork "github.com/Azure/azure-container-networking/network"
	"github.com/Azure/azure-container-networking/nns"
	"github.com/Azure/azure-container-networking/telemetry"
	cniSkel "github.com/containernetworking/cni/pkg/skel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	eth0IfName = "eth0"
)

var (
	args  *cniSkel.CmdArgs
	nwCfg cni.NetworkConfig
)

func TestMain(m *testing.M) {
	nwCfg = cni.NetworkConfig{
		Name:              "test-nwcfg",
		CNIVersion:        "0.3.0",
		Type:              "azure-vnet",
		Mode:              "bridge",
		Master:            eth0IfName,
		IPsToRouteViaHost: []string{"169.254.20.10"},
		Ipam: struct {
			Type          string `json:"type"`
			Environment   string `json:"environment,omitempty"`
			AddrSpace     string `json:"addressSpace,omitempty"`
			Subnet        string `json:"subnet,omitempty"`
			Address       string `json:"ipAddress,omitempty"`
			QueryInterval string `json:"queryInterval,omitempty"`
		}{
			Type: "azure-cns",
		},
	}

	args = &cniSkel.CmdArgs{
		ContainerID: "test-container",
		Netns:       "test-container",
	}
	args.StdinData = nwCfg.Serialize()
	podEnv := cni.K8SPodEnvArgs{
		K8S_POD_NAME:      "test-pod",
		K8S_POD_NAMESPACE: "test-pod-namespace",
	}
	args.Args = fmt.Sprintf("K8S_POD_NAME=%v;K8S_POD_NAMESPACE=%v", podEnv.K8S_POD_NAME, podEnv.K8S_POD_NAMESPACE)
	args.IfName = eth0IfName

	// Run tests.
	exitCode := m.Run()
	os.Exit(exitCode)
}

func GetTestResources() *NetPlugin {
	pluginName := "testplugin"
	config := &common.PluginConfig{}
	grpcClient := &nns.MockGrpcClient{}
	plugin, _ := NewPlugin(pluginName, config, grpcClient, &Multitenancy{}, nil)
	plugin.report = &telemetry.CNIReport{}
	mockNetworkManager := acnnetwork.NewMockNetworkmanager()
	plugin.nm = mockNetworkManager
	plugin.ipamInvoker = NewMockIpamInvoker(false, false, false)
	return plugin
}

// Check CNI returns error if required fields are missing
func TestPluginCNIFieldsMissing(t *testing.T) {
	plugin := GetTestResources()

	tests := []struct {
		name       string
		args       *cniSkel.CmdArgs
		plugin     *NetPlugin
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:   "Interface name not specified",
			plugin: plugin,
			args: &cniSkel.CmdArgs{
				StdinData:   nwCfg.Serialize(),
				ContainerID: "test-container",
				Netns:       "test-container",
				Args:        fmt.Sprintf("K8S_POD_NAME=%v;K8S_POD_NAMESPACE=%v", "test-pod", "test-pod-ns"),
			},
			wantErr:    true,
			wantErrMsg: "Interfacename not specified in CNI Args",
		},
		{
			name:   "Container ID not specified",
			plugin: plugin,
			args: &cniSkel.CmdArgs{
				StdinData: nwCfg.Serialize(),
				Netns:     "test-container",
				Args:      fmt.Sprintf("K8S_POD_NAME=%v;K8S_POD_NAMESPACE=%v", "test-pod", "test-pod-ns"),
				IfName:    eth0IfName,
			},
			wantErr:    true,
			wantErrMsg: "Container ID not specified in CNI Args",
		},
		{
			name:   "Pod Namespace not specified",
			plugin: plugin,
			args: &cniSkel.CmdArgs{
				StdinData:   nwCfg.Serialize(),
				Netns:       "test-container",
				ContainerID: "test-container",
				Args:        fmt.Sprintf("K8S_POD_NAME=%v;K8S_POD_NAMESPACE=%v", "test-pod", ""),
				IfName:      eth0IfName,
			},
			wantErr:    true,
			wantErrMsg: "Pod Namespace not specified in CNI Args",
		},
		{
			name:   "Pod Name not specified",
			plugin: plugin,
			args: &cniSkel.CmdArgs{
				StdinData:   nwCfg.Serialize(),
				Netns:       "test-container",
				ContainerID: "test-container",
				Args:        fmt.Sprintf("K8S_POD_NAME=%v;K8S_POD_NAMESPACE=%v", "", "test-pod-ns"),
				IfName:      eth0IfName,
			},
			wantErr:    true,
			wantErrMsg: "Pod Name not specified in CNI Args",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			err := plugin.Add(tt.args)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

/*
Multitenancy scenarios
*/

// Test Multitenancy Add
func TestPluginMultitenancyAdd(t *testing.T) {
	plugin, _ := cni.NewPlugin("test", "0.3.0")

	localNwCfg := cni.NetworkConfig{
		CNIVersion:                 "0.3.0",
		Name:                       "mulnet",
		MultiTenancy:               true,
		EnableExactMatchForPodName: true,
		Master:                     "eth0",
	}

	tests := []struct {
		name       string
		plugin     *NetPlugin
		args       *cniSkel.CmdArgs
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "Add Happy path",
			plugin: &NetPlugin{
				Plugin:             plugin,
				nm:                 acnnetwork.NewMockNetworkmanager(),
				tb:                 &telemetry.TelemetryBuffer{},
				report:             &telemetry.CNIReport{},
				multitenancyClient: NewMockMultitenancy(false),
			},
			args: &cniSkel.CmdArgs{
				StdinData:   localNwCfg.Serialize(),
				ContainerID: "test-container",
				Netns:       "test-container",
				Args:        fmt.Sprintf("K8S_POD_NAME=%v;K8S_POD_NAMESPACE=%v", "test-pod", "test-pod-ns"),
				IfName:      eth0IfName,
			},
			wantErr: false,
		},
		{
			name: "Add Fail",
			plugin: &NetPlugin{
				Plugin:             plugin,
				nm:                 acnnetwork.NewMockNetworkmanager(),
				tb:                 &telemetry.TelemetryBuffer{},
				report:             &telemetry.CNIReport{},
				multitenancyClient: NewMockMultitenancy(true),
			},
			args: &cniSkel.CmdArgs{
				StdinData:   localNwCfg.Serialize(),
				ContainerID: "test-container",
				Netns:       "test-container",
				Args:        fmt.Sprintf("K8S_POD_NAME=%v;K8S_POD_NAMESPACE=%v", "test-pod", "test-pod-ns"),
				IfName:      eth0IfName,
			},
			wantErr:    true,
			wantErrMsg: errMockMulAdd.Error(),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			err := tt.plugin.Add(tt.args)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrMsg, "Expected %v but got %+v", tt.wantErrMsg, err.Error())
			} else {
				require.NoError(t, err)
				endpoints, _ := tt.plugin.nm.GetAllEndpoints(localNwCfg.Name)
				require.Condition(t, assert.Comparison(func() bool { return len(endpoints) == 1 }))
			}
		})
	}
}

func TestPluginMultitenancyDelete(t *testing.T) {
	plugin := GetTestResources()
	plugin.multitenancyClient = NewMockMultitenancy(false)
	localNwCfg := cni.NetworkConfig{
		CNIVersion:                 "0.3.0",
		Name:                       "mulnet",
		MultiTenancy:               true,
		EnableExactMatchForPodName: true,
		Master:                     "eth0",
	}

	tests := []struct {
		name       string
		methods    []string
		args       *cniSkel.CmdArgs
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:    "Multitenancy delete success",
			methods: []string{CNI_ADD, CNI_DEL},
			args: &cniSkel.CmdArgs{
				StdinData:   localNwCfg.Serialize(),
				ContainerID: "test-container",
				Netns:       "test-container",
				Args:        fmt.Sprintf("K8S_POD_NAME=%v;K8S_POD_NAMESPACE=%v", "test-pod", "test-pod-ns"),
				IfName:      eth0IfName,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			var err error
			for _, method := range tt.methods {
				if method == CNI_ADD {
					err = plugin.Add(tt.args)
				} else if method == CNI_DEL {
					err = plugin.Delete(tt.args)
				}
			}
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				endpoints, _ := plugin.nm.GetAllEndpoints(localNwCfg.Name)
				require.Condition(t, assert.Comparison(func() bool { return len(endpoints) == 0 }))
			}
		})
	}
}

/*
Baremetal scenarios
*/

func TestPluginBaremetalAdd(t *testing.T) {
	plugin, _ := cni.NewPlugin("test", "0.3.0")

	localNwCfg := cni.NetworkConfig{
		CNIVersion:                 "0.3.0",
		Name:                       "baremetal-net",
		ExecutionMode:              string(Baremetal),
		EnableExactMatchForPodName: true,
		Master:                     "eth0",
	}

	tests := []struct {
		name       string
		plugin     *NetPlugin
		args       *cniSkel.CmdArgs
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "Baremetal Add Happy path",
			plugin: &NetPlugin{
				Plugin:    plugin,
				nm:        acnnetwork.NewMockNetworkmanager(),
				tb:        &telemetry.TelemetryBuffer{},
				report:    &telemetry.CNIReport{},
				nnsClient: &nns.MockGrpcClient{},
			},
			args: &cniSkel.CmdArgs{
				StdinData:   localNwCfg.Serialize(),
				ContainerID: "test-container",
				Netns:       "test-container",
				Args:        fmt.Sprintf("K8S_POD_NAME=%v;K8S_POD_NAMESPACE=%v", "test-pod", "test-pod-ns"),
				IfName:      eth0IfName,
			},
			wantErr: false,
		},
		{
			name: "Baremetal Add Fail",
			plugin: &NetPlugin{
				Plugin:    plugin,
				nm:        acnnetwork.NewMockNetworkmanager(),
				tb:        &telemetry.TelemetryBuffer{},
				report:    &telemetry.CNIReport{},
				nnsClient: &nns.MockGrpcClient{Fail: true},
			},
			args: &cniSkel.CmdArgs{
				StdinData:   localNwCfg.Serialize(),
				ContainerID: "test-container",
				Netns:       "test-container",
				Args:        fmt.Sprintf("K8S_POD_NAME=%v;K8S_POD_NAMESPACE=%v", "test-pod", "test-pod-ns"),
				IfName:      eth0IfName,
			},
			wantErr:    true,
			wantErrMsg: nns.ErrMockNnsAdd.Error(),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			err := tt.plugin.Add(tt.args)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrMsg, "Expected %v but got %+v", tt.wantErrMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPluginBaremetalDelete(t *testing.T) {
	plugin := GetTestResources()
	plugin.nnsClient = &nns.MockGrpcClient{}
	localNwCfg := cni.NetworkConfig{
		CNIVersion:                 "0.3.0",
		Name:                       "baremetal-net",
		ExecutionMode:              string(Baremetal),
		EnableExactMatchForPodName: true,
		Master:                     "eth0",
	}

	tests := []struct {
		name       string
		methods    []string
		args       *cniSkel.CmdArgs
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:    "Baremetal delete success",
			methods: []string{CNI_ADD, CNI_DEL},
			args: &cniSkel.CmdArgs{
				StdinData:   localNwCfg.Serialize(),
				ContainerID: "test-container",
				Netns:       "test-container",
				Args:        fmt.Sprintf("K8S_POD_NAME=%v;K8S_POD_NAMESPACE=%v", "test-pod", "test-pod-ns"),
				IfName:      eth0IfName,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			var err error
			for _, method := range tt.methods {
				if method == CNI_ADD {
					err = plugin.Add(tt.args)
				} else if method == CNI_DEL {
					err = plugin.Delete(tt.args)
				}
			}

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				endpoints, _ := plugin.nm.GetAllEndpoints(localNwCfg.Name)
				require.Condition(t, assert.Comparison(func() bool { return len(endpoints) == 0 }))
			}
		})
	}
}

func TestNewPlugin(t *testing.T) {
	tests := []struct {
		name    string
		config  common.PluginConfig
		wantErr bool
	}{
		{
			name: "Test new plugin",
			config: common.PluginConfig{
				Version: "0.3.0",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			plugin, err := NewPlugin("test", &tt.config, nil, nil, nil)
			if tt.wantErr {
				require.NoError(t, err)
				require.NotNil(t, plugin)
			}

			err = plugin.Start(&tt.config)
			if tt.wantErr {
				require.NoError(t, err)
			}

			plugin.Stop()
		})
	}
}

func getTestEndpoint(podname, podnamespace, ipwithcidr, podinterfaceid, infracontainerid string) *acnnetwork.EndpointInfo {
	ip, ipnet, _ := net.ParseCIDR(ipwithcidr)
	ipnet.IP = ip
	ep := acnnetwork.EndpointInfo{
		PODName:      podname,
		PODNameSpace: podnamespace,
		Id:           podinterfaceid,
		ContainerID:  infracontainerid,
		IPAddresses: []net.IPNet{
			*ipnet,
		},
	}

	return &ep
}

func TestGetAllEndpointState(t *testing.T) {
	plugin := GetTestResources()
	networkid := "azure"

	ep1 := getTestEndpoint("podname1", "podnamespace1", "10.0.0.1/24", "podinterfaceid1", "testcontainerid1")
	ep2 := getTestEndpoint("podname2", "podnamespace2", "10.0.0.2/24", "podinterfaceid2", "testcontainerid2")

	err := plugin.nm.CreateEndpoint(nil, networkid, ep1)
	require.NoError(t, err)

	err = plugin.nm.CreateEndpoint(nil, networkid, ep2)
	require.NoError(t, err)

	state, err := plugin.GetAllEndpointState(networkid)
	require.NoError(t, err)

	res := &api.AzureCNIState{
		ContainerInterfaces: map[string]api.PodNetworkInterfaceInfo{
			ep1.Id: {
				PodEndpointId: ep1.Id,
				PodName:       ep1.PODName,
				PodNamespace:  ep1.PODNameSpace,
				ContainerID:   ep1.ContainerID,
				IPAddresses:   ep1.IPAddresses,
			},
			ep2.Id: {
				PodEndpointId: ep2.Id,
				PodName:       ep2.PODName,
				PodNamespace:  ep2.PODNameSpace,
				ContainerID:   ep2.ContainerID,
				IPAddresses:   ep2.IPAddresses,
			},
		},
	}

	require.Exactly(t, res, state)
}

func TestEndpointsWithEmptyState(t *testing.T) {
	plugin := GetTestResources()
	networkid := "azure"
	state, err := plugin.GetAllEndpointState(networkid)
	require.NoError(t, err)
	require.Equal(t, 0, len(state.ContainerInterfaces))
}

func TestGetNetworkName(t *testing.T) {
	plugin := GetTestResources()
	tests := []struct {
		name  string
		nwCfg cni.NetworkConfig
	}{
		{
			name: "get network name",
			nwCfg: cni.NetworkConfig{
				Name: "test-network",
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			nwName, _ := plugin.getNetworkName("", "", "", &tt.nwCfg)
			require.Equal(t, tt.nwCfg.Name, nwName)
		})
	}
}
