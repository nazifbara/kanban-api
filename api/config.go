package api

import (
	"database/sql"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/nazifbara/kanban-api/internal/database"
)

type ApiConfig struct {
	Port      string
	DBQueries *database.Queries
}

func NewAPIConfig() ApiConfig {
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

	return ApiConfig{
		Port:      port,
		DBQueries: dbQueries,
	}
}
