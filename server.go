package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/nazifbara/kanban-api/internal/database"
)

type contextKey string

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
		Handler: requestLogger(logger)(mux),
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

type spyReadCloser struct {
	io.ReadCloser
	bytesRead int
}

func (s *spyReadCloser) Read(p []byte) (int, error) {
	n, err := s.ReadCloser.Read(p)
	s.bytesRead += n
	return n, err
}

type spyResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (s *spyResponseWriter) Write(p []byte) (int, error) {
	if s.statusCode == 0 {
		s.statusCode = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(p)
	s.bytesWritten += n
	return n, err
}

func (s *spyResponseWriter) WriteHeader(code int) {
	s.statusCode = code
	s.ResponseWriter.WriteHeader(code)
}

var logContextKey contextKey = "log_context"

type LogContext struct {
	Error error
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			spyReader := &spyReadCloser{ReadCloser: r.Body}
			r.Body = spyReader
			spyRespond := &spyResponseWriter{ResponseWriter: w}
			logCtx := &LogContext{}
			r = r.WithContext(context.WithValue(r.Context(), logContextKey, logCtx))

			next.ServeHTTP(spyRespond, r)

			slogAttrs := []any{
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Duration("duration", time.Since(start)),
				slog.Int("request_body_bytes", spyReader.bytesRead),
				slog.Int("response_body_bytes", spyRespond.bytesWritten),
				slog.Int("response_status", spyRespond.statusCode),
			}
			if logCtx.Error != nil {
				slogAttrs = append(slogAttrs, slog.Any("error", logCtx.Error))
			}

			logger.Info("served request", slogAttrs...)
		})
	}
}
