package nmagentclient

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/Azure/azure-container-networking/cns/logger"
	"github.com/Azure/azure-container-networking/common"
)

// JoinNetwork joins the given network
func JoinNetwork(
	networkID string,
	joinNetworkURL string) (*http.Response, error) {
	logger.Printf("[NMAgentClient] JoinNetwork: %s", networkID)

	// Empty body is required as wireserver cannot handle a post without the body.
	var body bytes.Buffer
	json.NewEncoder(&body).Encode("")
	response, err := common.GetHttpClient().Post(joinNetworkURL, "application/json", &body)

	if err == nil && response.StatusCode == http.StatusOK {
		defer response.Body.Close()
	}

	logger.Printf("[NMAgentClient][Response] Join network: %s. Response: %+v. Error: %v",
		networkID, response, err)

	return response, err
}

// PublishNetworkContainer publishes given network container
func PublishNetworkContainer(
	networkContainerID string,
	createNetworkContainerURL string,
	requestBodyData []byte) (*http.Response, error) {
	logger.Printf("[NMAgentClient] PublishNetworkContainer NC: %s", networkContainerID)

	requestBody := bytes.NewBuffer(requestBodyData)
	response, err := common.GetHttpClient().Post(createNetworkContainerURL, "application/json", requestBody)

	logger.Printf("[NMAgentClient][Response] Publish NC: %s. Response: %+v. Error: %v",
		networkContainerID, response, err)

	return response, err
}

// UnpublishNetworkContainer unpublishes given network container
func UnpublishNetworkContainer(
	networkContainerID string,
	deleteNetworkContainerURL string) (*http.Response, error) {
	logger.Printf("[NMAgentClient] UnpublishNetworkContainer NC: %s", networkContainerID)

	// Empty body is required as wireserver cannot handle a post without the body.
	var body bytes.Buffer
	json.NewEncoder(&body).Encode("")
	response, err := common.GetHttpClient().Post(deleteNetworkContainerURL, "application/json", &body)

	logger.Printf("[NMAgentClient][Response] Unpublish NC: %s. Response: %+v. Error: %v",
		networkContainerID, response, err)

	return response, err
}
