package main

import (
	"log"
	"net/http"

	_ "github.com/lib/pq"
	"github.com/nazifbara/kanban-api/internal/apiconfig"
)

func main() {
	apiConfig := apiconfig.NewAPIConfig()
	mux := http.NewServeMux()

	server := http.Server{
		Addr:    ":" + apiConfig.Port,
		Handler: mux,
	}

	log.Printf("Listening for requests on port %s", apiConfig.Port)
	log.Fatal(server.ListenAndServe())
}
