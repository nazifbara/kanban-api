package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/nazifbara/kanban-api/internal/apierrors"
)

var malformedBodyErr error = apierrors.New(http.StatusBadRequest, "malformed request body")

type contextKey string

type server struct {
	httpServer *http.Server
	store      *store
	logger     *slog.Logger
	cancel     context.CancelFunc
	jwtSecret  string
}

func newServer(port int, store *store, logger *slog.Logger, cancel context.CancelFunc) *server {
	mux := http.NewServeMux()
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           requestLogger(logger)(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET must be set")
	}
	s := &server{
		httpServer: srv,
		jwtSecret:  jwtSecret,
		store:      store,
		logger:     logger,
		cancel:     cancel,
	}

	mux.HandleFunc("POST /api/refresh", s.handlerRefresh)
	mux.HandleFunc("POST /api/login", s.handlerLogin)
	mux.HandleFunc("POST /api/sign-up", s.handlerSignUp)
	mux.Handle("POST /api/boards", s.withAuth(s.handlerCreateBoard))
	mux.Handle("GET /api/boards", s.withAuth(s.handlerGetAllBoards))
	mux.Handle("GET /api/boards/{boardID}", s.withAuth(s.withBoardAccess(s.handlerGetBoard)))
	mux.Handle("DELETE /api/boards/{boardID}", s.withAuth(s.withBoardAccess(s.handlerDeleteBoard)))
	mux.Handle("PUT /api/boards/{boardID}", s.withAuth(s.withBoardAccess(s.hanlderUpdateBoard)))
	mux.Handle("POST /api/boards/{boardID}/columns", s.withAuth(s.withBoardAccess(s.handlerCreateColumn)))
	mux.Handle("GET /api/boards/{boardID}/columns", s.withAuth(s.withBoardAccess(s.handlerBoardColumns)))
	mux.Handle("DELETE /api/boards/{boardID}/columns/{columnID}", s.withAuth(s.withBoardAccess(s.withColumnAccess(s.handlerDeleteColumn))))
	mux.Handle("PATCH /api/boards/{boardID}/columns/{columnID}", s.withAuth(s.withBoardAccess(s.withColumnAccess(s.handlerPatchColumn))))
	mux.Handle("POST /api/boards/{boardID}/columns/{columnID}/tasks", s.withAuth(s.withBoardAccess(s.withColumnAccess(s.handlerCreateTask))))
	mux.Handle("PATCH /api/boards/{boardID}/tasks/{taskID}", s.withAuth(s.withBoardAccess(s.handlerUpdateTask)))
	mux.Handle("GET /api/boards/{boardID}/columns/{columnID}/tasks", s.withAuth(s.withBoardAccess(s.withColumnAccess(s.handlerColumnTasks))))
	mux.Handle("GET /api/boards/{boardID}/tasks", s.withAuth(s.withBoardAccess(s.handlerGetBoardTasks)))
	mux.Handle("DELETE /api/boards/{boardID}/tasks/{taskID}", s.withAuth(s.withBoardAccess(s.handlerDeleteTask)))
	mux.HandleFunc("POST /reset", s.handlerReset)

	return s
}

func (s *server) handlerReset(w http.ResponseWriter, r *http.Request) {
	if err := s.store.TruncateIdentities(r.Context()); err != nil {
		w.WriteHeader(500)
		return
	}
	w.WriteHeader(200)
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
