package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

type Data struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

func homePage(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Welcome to the HomePage!")
	fmt.Println("Endpoint Hit: homePage")
}

func handleRequests() {
	router := mux.NewRouter().StrictSlash(true)
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},   // All origins
		AllowedMethods: []string{"GET"}, // Allowing only get, just an example
	})
	router.HandleFunc("/", homePage)
	router.HandleFunc("/getRules", getRules).Methods("GET", "OPTIONS")
	log.Fatal(http.ListenAndServe(":10091", c.Handler(router)))
}

func getRules(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Endpoint Hit: getRules")
	src, ok := r.URL.Query()["src"]
	if !ok || len(src[0]) < 1 {
		log.Println("Url Param 'src' is missing")
		return
	}
	dst, ok := r.URL.Query()["dst"]
	if !ok || len(src[0]) < 1 {
		log.Println("Url Param 'dst' is missing")
		return
	}
	source, destination := src[0], dst[0]
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	data := Data{}
	data.Source = source
	data.Destination = destination
	json.NewEncoder(w).Encode(data)
}

func main() {
	fmt.Println("Network Visualizer API v1.0")
	handleRequests()
}
