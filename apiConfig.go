package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type apiConfig struct {
	port string
}

func newAPIConfig() apiConfig {
	godotenv.Load()

	port := os.Getenv("API_PORT")
	if port == "" {
		log.Fatal("API_PORT must be set")
	}

	return apiConfig{
		port: port,
	}
}
