package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/spread/thousand_sunny/internal/config"
	"github.com/spread/thousand_sunny/internal/handlers"
	"github.com/spread/thousand_sunny/internal/migrate"
	"github.com/spread/thousand_sunny/internal/store"
)

func main() {
	cfg := config.Load()

	for _, dir := range []string{
		filepath.Join(cfg.MediaRoot, "videos"),
		filepath.Join(cfg.MediaRoot, "posters"),
		filepath.Join(cfg.MediaRoot, "series_posters"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatalf("create media dir: %v", err)
		}
	}

	ctx := context.Background()
	pool, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	if err := migrate.Up(ctx, pool); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	api := &handlers.API{
		Store:     store.New(pool),
		MediaRoot: cfg.MediaRoot,
	}
	api.QueuePendingTranscodes()

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Mount("/api", api.Routes())

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("Thousand Sunny API listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
