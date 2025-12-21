package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wyg1997/LedgerBot/config"
	"github.com/wyg1997/LedgerBot/internal/infrastructure/ai"
	"github.com/wyg1997/LedgerBot/internal/infrastructure/platform/feishu"
	"github.com/wyg1997/LedgerBot/internal/infrastructure/repository"
	"github.com/wyg1997/LedgerBot/internal/interfaces/http/handler"
	"github.com/wyg1997/LedgerBot/internal/usecase"
	"github.com/wyg1997/LedgerBot/pkg/logger"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()
	if err := cfg.IsValid(); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid configuration: %v\n", err)
		os.Exit(1)
	}

	// Set log level
	logger.SetLogLevel(cfg.Storage.LogLevel)
	log := logger.GetLogger()

	log.Info("Starting Ledger Bot...")

	// Initialize services
	feishuService := feishu.NewFeishuService(&cfg.Feishu)
	aiService := ai.NewOpenAIService(&cfg.AI)

	// Initialize repositories
	userMappingRepo, err := repository.NewUserMappingRepository(cfg.Storage.DataDir)
	if err != nil {
		log.Fatal("Failed to create user mapping repository: %v", err)
	}

	billRepo, err := repository.NewBitableBillRepository(feishuService, &cfg.Feishu)
	if err != nil {
		log.Fatal("Failed to create bill repository: %v", err)
	}

	// Initialize use cases
	billUseCase := usecase.NewBillUseCase(billRepo, userMappingRepo)

	// Initialize handlers
	feishuHandler := handler.NewFeishuHandlerAITools(&cfg.Feishu, feishuService, billUseCase, aiService, userMappingRepo)

	// Create HTTP server
	mux := http.NewServeMux()

	// Feishu webhook endpoint
	mux.HandleFunc("/webhook/feishu", feishuHandler.Webhook)

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Create server
	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      mux,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Info("Server starting on port %s", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("Server forced to shutdown: %v", err)
	}

	log.Info("Server exited")
}