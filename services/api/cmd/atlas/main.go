package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	_ "modernc.org/sqlite"
)

func main() {
	dbPath := env("ATLAS_DB", filepath.Join("..", "..", "data", "index.sqlite"))
	abs, err := filepath.Abs(dbPath)
	if err != nil {
		log.Fatalf("resolve database path: %v", err)
	}
	if _, err = os.Stat(abs); err != nil {
		log.Fatalf("scenario index %s is not available; run `make demo-data` or import a reviewed metadata release: %v", abs, err)
	}
	db, err := sql.Open("sqlite", "file:"+abs+"?mode=ro&_pragma=busy_timeout(5000)")
	if err != nil {
		log.Fatalf("open read-only scenario index: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(4)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("read scenario index: %v", err)
	}
	service := &service{db: db, webRoot: env("ATLAS_WEB_ROOT", "../../apps/web/dist")}
	server := &http.Server{Addr: env("ATLAS_ADDR", "127.0.0.1:8080"), Handler: logging(service.routes()), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	stopped := make(chan os.Signal, 1)
	signal.Notify(stopped, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stopped
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	log.Printf("atlas API listening on %s; index=%s", server.Addr, abs)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
