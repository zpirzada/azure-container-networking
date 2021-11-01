package ipsets

import (
	testutils "github.com/Azure/azure-container-networking/test/utils"

	"github.com/Azure/azure-container-networking/network/hnswrapper"
	"github.com/Microsoft/hcsshim/hcn"
)

func GetApplyIPSetsTestCalls(_, _ []*IPSetMetadata) []testutils.TestCmd {
	return []testutils.TestCmd{}
}

func GetResetTestCalls() []testutils.TestCmd {
	return []testutils.TestCmd{}
}

func GetHNSFake() *hnswrapper.Hnsv2wrapperFake {
	hns := hnswrapper.NewHnsv2wrapperFake()
	network := &hcn.HostComputeNetwork{
		Id:   "1234",
		Name: "azure",
	}

	hns.CreateNetwork(network)

	return hns
}
