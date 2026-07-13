package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/nazifbara/kanban-api/internal/database"
)

func main() {
	godotenv.Load()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	httpPort := flag.Int("port", 8080, "port to listen on")
	flag.Parse()
	status := run(ctx, cancel, *httpPort)
	cancel()
	os.Exit(status)
}

func run(ctx context.Context, cancel context.CancelFunc, httpPort int) int {
	dbQueries, err := initializeDB(os.Getenv("DB_URL"))
	if err != nil {
		log.Printf("failed to connect to DB: %v", err)
	}
	s := newServer(httpPort, dbQueries, cancel)
	var serverError error
	go func() {
		serverError = s.start()
		cancel()
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	log.Println("kanban api is shutting down")
	if err := s.shutdown(shutdownCtx); err != nil {
		log.Printf("failed to shutdown the server: %v", err)
		return 1
	}
	if serverError != nil {
		log.Printf("server error: %v", serverError)
		return 1
	}
	return 0
}

func initializeDB(dbURL string) (*database.Queries, error) {
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, err
	}
	return database.New(db), nil
}
