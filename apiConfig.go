package main

import (
	"database/sql"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/nazifbara/kanban-api/internal/database"
)

type apiConfig struct {
	port      string
	dbQueries *database.Queries
}

func newAPIConfig() apiConfig {
	godotenv.Load()

	port := os.Getenv("API_PORT")
	if port == "" {
		log.Fatal("API_PORT must be set")
	}
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		log.Fatal("DB_URL must be set")
	}
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	log.Println("Database connection established!")
	dbQueries := database.New(db)

	return apiConfig{
		port:      port,
		dbQueries: dbQueries,
	}
}
