package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/nazifbara/kanban-api/internal/database"
	pkgerr "github.com/pkg/errors"
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
	logger := initializeLogger()
	dbQueries, err := initializeDB(os.Getenv("DB_URL"))
	if err != nil {
		logger.Error(fmt.Sprintf("failed to connect to DB: %v", err))
	}
	s := newServer(httpPort, dbQueries, logger, cancel)
	var serverError error
	go func() {
		serverError = s.start()
		cancel()
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	logger.Debug("kanban api is shutting down")
	if err := s.shutdown(shutdownCtx); err != nil {
		logger.Error(fmt.Sprintf("failed to shutdown the server: %v", err))
		return 1
	}
	if serverError != nil {
		logger.Error(fmt.Sprintf("server error: %v", serverError))
		return 1
	}
	return 0
}

func initializeStore(dbURL string) (*store, error) {
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, err
	}

	return &store{Queries: database.New(db), db: db}, nil
}

func initializeLogger() *slog.Logger {
	handler := tint.NewTextHandler(os.Stderr, &tint.Options{
		Level:       slog.LevelDebug,
		ReplaceAttr: replaceLogAttr,
		NoColor:     !(isatty.IsCygwinTerminal(os.Stderr.Fd()) || isatty.IsTerminal(os.Stderr.Fd())),
	})
	return slog.New(handler)
}

type stackTracker interface {
	error
	StackTrace() pkgerr.StackTrace
}

func errorAttrs(err error) []slog.Attr {
	attrs := []slog.Attr{
		slog.String("message", err.Error()),
	}
	if stackErr, ok := errors.AsType[stackTracker](err); ok {
		attrs = append(attrs, slog.String("stack_trace", fmt.Sprintf("%+v", stackErr.StackTrace())))
	}
	return attrs
}

func replaceLogAttr(groups []string, a slog.Attr) slog.Attr {
	if a.Key == "error" {
		err, ok := a.Value.Any().(error)
		if !ok {
			return a
		}
		return slog.GroupAttrs("error", errorAttrs(err)...)
	}
	return a
}
