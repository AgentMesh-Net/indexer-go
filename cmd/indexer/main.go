package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AgentMesh-Net/indexer-go/internal/api"
	"github.com/AgentMesh-Net/indexer-go/internal/config"
	"github.com/AgentMesh-Net/indexer-go/internal/store"
	"github.com/AgentMesh-Net/indexer-go/migrations"
)

func main() {
	cfg := config.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := store.NewPool(ctx, cfg.DBDSN)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer pool.Close()

	migrationSQL, err := migrations.FS.ReadFile("001_init.sql")
	if err != nil {
		log.Fatalf("read migration file: %v", err)
	}
	if err := store.RunMigrations(ctx, pool, string(migrationSQL)); err != nil {
		log.Fatalf("migration failed: %v", err)
	}
	log.Println("migrations applied")

	repo := store.NewPostgresRepo(pool)
	router := api.NewRouter(repo, cfg.MaxBodyBytes)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1MB
	}

	go func() {
		log.Printf("indexer listening on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
	log.Println("server stopped")
}
