// Copyright Microsoft Corp.
// All rights reserved.

package core

import (
	"encoding/json"
    "fmt"
    "io"
	"log"
	"net"
    "net/http"
	"os"
	"path"
)

// Libnetwork plugin path.
const pluginPath = "/run/docker/plugins"

// Listener object.
type Listener struct {
	socketName string
    endpoints []string
    l net.Listener
    mux *http.ServeMux
}

// Creates a new Listener.
func NewListener(name string) (*Listener, error) {
    var listener Listener

	listener.socketName = path.Join(pluginPath, name) + ".sock"
    listener.mux = http.NewServeMux()

    return &listener, nil
}

// Starts listening for requests from libnetwork and routes them to the corresponding plugin.
func (listener *Listener) Start() error {
    var err error

    // Create a socket.
    os.MkdirAll(pluginPath, 0660)

	listener.l, err = net.Listen("unix", listener.socketName)
	if err != nil {
        log.Fatalf("Listener: Failed to listen on %s %v", listener.socketName, err)
	}

    // Register internal handlers.
    listener.AddHandler("Plugin", "Activate", listener.activate)
    listener.AddHandler("", "status", listener.status)

    log.Printf("Listener: Started listening on %s.", listener.socketName)

    return http.Serve(listener.l, listener.mux)
}

// Stops listening for requests from libnetwork.
func (listener *Listener) Stop() {

    // Stop servicing requests.
	listener.l.Close()

    // Delete the socket.
    os.Remove(listener.socketName)

    log.Printf("Listener: Stopped listening on %s", listener.socketName)
}

// Register an object.
func (listener *Listener) AddEndpoint(endpoint string) {
    listener.endpoints = append(listener.endpoints, endpoint)
}

// Register the handler for an object.
func (listener *Listener) AddHandler(endpoint string, method string, handler func(http.ResponseWriter, *http.Request)) {
    url := fmt.Sprintf("/%s.%s", endpoint, method)
    listener.mux.HandleFunc(url, handler)
}

// Decode JSON payload.
func (listener *Listener) Decode(w http.ResponseWriter, r *http.Request, request interface{}) error {
	err := json.NewDecoder(r.Body).Decode(request)
	if err != nil {
        http.Error(w, "Unable to decode JSON payload: " + err.Error(), http.StatusBadRequest)
	}
	return err
}

// Encode JSON payload.
func (listener *Listener) Encode(w http.ResponseWriter, response interface{}) error {
    err := json.NewEncoder(w).Encode(response)
    if err != nil {
        http.Error(w, "Unable to encode JSON payload: " + err.Error(), http.StatusInternalServerError)
    }
	return err
}

// Activation response sent to libnetwork remote driver.
type activateResponse struct {
    Implements []string
}

func (listener *Listener) activate(w http.ResponseWriter, r *http.Request) {
	var resp activateResponse

    log.Printf("Listener: Received activate request.")

    resp.Implements = listener.endpoints

    err := listener.Encode(w, resp)
    if err != nil {
        fmt.Println("Listener: Failed to encode activation response.")
        return
    }

    log.Printf("Listener: Responded with activate response %v.", resp)
}

func (listener *Listener) status(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, fmt.Sprintln("azure network plugin", "V0"))
}
