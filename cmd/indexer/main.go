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
	"github.com/AgentMesh-Net/indexer-go/internal/chain"
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

	for _, migFile := range []string{"001_init.sql", "002_tasks.sql", "003_onchain_sync.sql"} {
		migrationSQL, err := migrations.FS.ReadFile(migFile)
		if err != nil {
			log.Fatalf("read migration file %s: %v", migFile, err)
		}
		if err := store.RunMigrations(ctx, pool, string(migrationSQL)); err != nil {
			log.Fatalf("migration %s failed: %v", migFile, err)
		}
		log.Printf("migration %s applied", migFile)
	}

	repo := store.NewPostgresRepo(pool)
	taskRepo := store.NewPostgresTaskRepo(pool)
	router := api.NewRouter(repo, taskRepo, cfg)

	// B4: Start one watcher goroutine per configured chain
	for _, chainCfg := range cfg.SupportedChains {
		rpcURL, ok := cfg.RPCURLs[chainCfg.ChainID]
		if !ok || rpcURL == "" {
			log.Printf("no RPC URL configured for chain %d — watcher disabled", chainCfg.ChainID)
			continue
		}
		w, err := chain.NewWatcher(rpcURL, chainCfg, taskRepo)
		if err != nil {
			log.Printf("failed to create watcher for chain %d: %v — skipping", chainCfg.ChainID, err)
			continue
		}
		go w.Run(ctx)
		log.Printf("chain watcher started for chain=%d contract=%s", chainCfg.ChainID, chainCfg.SettlementContract)
	}

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
