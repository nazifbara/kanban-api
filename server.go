package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/nazifbara/kanban-api/internal/database"
)

type server struct {
	httpServer *http.Server
	dbQueries  *database.Queries
	logger     *slog.Logger
	cancel     context.CancelFunc
}

func newServer(port int, dbQueries *database.Queries, logger *slog.Logger, cancel context.CancelFunc) *server {
	mux := http.NewServeMux()
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}
	s := &server{
		httpServer: srv,
		dbQueries:  dbQueries,
		logger:     logger,
		cancel:     cancel,
	}

	mux.HandleFunc("POST /api/boards", s.handlerCreateBoard)
	mux.HandleFunc("GET /api/boards", s.handlerGetAllBoards)
	mux.HandleFunc("GET /api/boards/{boardID}", s.handlerGetBoard)
	mux.HandleFunc("DELETE /api/boards/{boardID}", s.handlerDeleteBoard)
	mux.HandleFunc("PUT /api/boards/{boardID}", s.hanlderUpdateBoard)
	mux.HandleFunc("POST /api/states", s.handlerCreateState)

	return s
}

func (s *server) start() error {
	ln, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return err
	}
	s.logger.Debug(fmt.Sprintf("Kanban API is running on http://localhost:%d", ln.Addr().(*net.TCPAddr).Port))
	if err := s.httpServer.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *server) shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
