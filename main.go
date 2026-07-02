package main

import (
	"log"
	"net/http"
)

func main() {
	apiConfig := newAPIConfig()
	mux := http.NewServeMux()

	server := http.Server{
		Addr:    ":" + apiConfig.port,
		Handler: mux,
	}

	log.Printf("Listening for requests on port %s", apiConfig.port)
	log.Fatal(server.ListenAndServe())
}
