// Copyright Microsoft Corp.
// All rights reserved.

package core

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path"
)

// Libnetwork plugin path
const pluginPath = "/run/docker/plugins"

// Listener object
type Listener struct {
	socketName string
	l          net.Listener
	mux        *http.ServeMux
}

// Creates a new Listener.
func NewListener(name string) (*Listener, error) {
	var listener Listener

	listener.socketName = path.Join(pluginPath, name) + ".sock"
	listener.mux = http.NewServeMux()

	return &listener, nil
}

// Starts listening for requests from libnetwork and routes them to the corresponding plugin.
func (listener *Listener) Start(errChan chan error) error {
	var err error

	// Create a socket.
	os.MkdirAll(pluginPath, 0660)

	listener.l, err = net.Listen("unix", listener.socketName)
	if err != nil {
		log.Fatalf("Listener: Failed to listen on %s %v", listener.socketName, err)
	}

	log.Printf("Listener: Started listening on %s.", listener.socketName)

	go func() {
		errChan <- http.Serve(listener.l, listener.mux)
	}()

	return nil
}

// Stops listening for requests from libnetwork.
func (listener *Listener) Stop() {

	// Stop servicing requests.
	listener.l.Close()

	// Delete the socket.
	os.Remove(listener.socketName)

	log.Printf("Listener: Stopped listening on %s", listener.socketName)
}

// Registers a protocol handler.
func (listener *Listener) AddHandler(endpoint string, method string, handler func(http.ResponseWriter, *http.Request)) {
	url := fmt.Sprintf("/%s.%s", endpoint, method)
	listener.mux.HandleFunc(url, handler)
}

// Decodes JSON payload.
func (listener *Listener) Decode(w http.ResponseWriter, r *http.Request, request interface{}) error {
	err := json.NewDecoder(r.Body).Decode(request)
	if err != nil {
		http.Error(w, "Failed to decode request: "+err.Error(), http.StatusBadRequest)
		log.Println("Listener: Failed to decode request: " + err.Error())
	}
	return err
}

// Encodes JSON payload.
func (listener *Listener) Encode(w http.ResponseWriter, response interface{}) error {
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		http.Error(w, "Failed to encode response: "+err.Error(), http.StatusInternalServerError)
		log.Println("Listener: Failed to encode response: " + err.Error())
	}
	return err
}

// Sends an error response.
func (listener *Listener) SendError(w http.ResponseWriter, errMessage string) {
	json.NewEncoder(w).Encode(map[string]string{"Err": errMessage})
}
