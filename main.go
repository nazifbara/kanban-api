package main

import (
	"log"
	"net/http"

	_ "github.com/lib/pq"
	"github.com/nazifbara/kanban-api/api"
)

func main() {
	apiConfig := api.NewAPIConfig()
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/boards", apiConfig.HandlerCreateBoard)
	mux.HandleFunc("GET /api/boards", apiConfig.HandlerGetAllBoards)
	mux.HandleFunc("GET /api/boards/{boardID}", apiConfig.HandlerGetBoard)

	server := http.Server{
		Addr:    ":" + apiConfig.Port,
		Handler: mux,
	}

	log.Printf("Listening for requests on port %s", apiConfig.Port)
	log.Fatal(server.ListenAndServe())
}
